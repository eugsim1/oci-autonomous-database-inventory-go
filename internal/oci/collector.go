package oci

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/eugsim1/oci-autonomous-database-inventory-go/internal/config"
	"github.com/eugsim1/oci-autonomous-database-inventory-go/internal/model"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/database"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/oracle/oci-go-sdk/v65/resourcesearch"
)

type Collector struct {
	cfg         config.Config
	provider    common.ConfigurationProvider
	tenancyID   string
	logOutput   io.Writer
	clientCache *databaseClientCache
}

type resourceRef struct {
	Region string
	ID     string
}

func NewCollector(cfg config.Config, logOutput io.Writer) (*Collector, error) {
	provider, tenancyID, err := configurationProvider(cfg)
	if err != nil {
		return nil, err
	}
	return &Collector{
		cfg:         cfg,
		provider:    provider,
		tenancyID:   tenancyID,
		logOutput:   logOutput,
		clientCache: newDatabaseClientCache(provider),
	}, nil
}

func (c *Collector) Collect(ctx context.Context) (model.Report, error) {
	report := model.Report{
		SchemaVersion:  "1.0",
		GeneratedAt:    time.Now().UTC(),
		TenancyOCID:    c.tenancyID,
		Authentication: c.cfg.AuthMode,
		SearchQuery:    model.SearchQuery,
		Databases:      []model.DatabaseRecord{},
	}

	regions, err := c.listRegions(ctx)
	if err != nil {
		return report, err
	}
	scannedRegions, err := selectRegions(regions, c.cfg.Regions)
	if err != nil {
		return report, err
	}
	scannedSet := make(map[string]struct{}, len(scannedRegions))
	for _, region := range scannedRegions {
		scannedSet[region] = struct{}{}
	}
	for i := range regions {
		_, regions[i].Scanned = scannedSet[regions[i].Name]
	}
	report.SubscribedRegions = regions

	fmt.Fprintf(c.logOutput, "tenancy: %s\n", c.tenancyID)
	fmt.Fprintf(c.logOutput, "READY regions to scan: %s\n", strings.Join(scannedRegions, ", "))

	refs, searchErrors := c.searchAllRegions(ctx, scannedRegions)
	report.Errors = append(report.Errors, searchErrors...)
	fmt.Fprintf(c.logOutput, "OCI Search discovered %d Autonomous Database resource(s)\n", len(refs))

	records, detailErrors := c.getAllDatabases(ctx, refs)
	report.Databases = append(report.Databases, records...)
	report.Errors = append(report.Errors, detailErrors...)
	report.Finalize()
	return report, nil
}

func (c *Collector) listRegions(ctx context.Context) ([]model.Region, error) {
	client, err := identity.NewIdentityClientWithConfigurationProvider(c.provider)
	if err != nil {
		return nil, fmt.Errorf("create Identity client: %w", err)
	}
	if c.cfg.BootstrapRegion != "" {
		client.SetRegion(c.cfg.BootstrapRegion)
	}
	response, err := client.ListRegionSubscriptions(ctx, identity.ListRegionSubscriptionsRequest{
		TenancyId:       common.String(c.tenancyID),
		RequestMetadata: retryMetadata(),
	})
	if err != nil {
		return nil, fmt.Errorf("list subscribed regions: %w", err)
	}

	regions := make([]model.Region, 0, len(response.Items))
	for _, item := range response.Items {
		regions = append(regions, model.Region{
			Name:         strings.ToLower(value(item.RegionName)),
			Key:          value(item.RegionKey),
			Status:       string(item.Status),
			IsHomeRegion: boolValue(item.IsHomeRegion),
		})
	}
	return regions, nil
}

func selectRegions(regions []model.Region, requested []string) ([]string, error) {
	ready := make(map[string]bool, len(regions))
	for _, region := range regions {
		ready[region.Name] = strings.EqualFold(region.Status, string(identity.RegionSubscriptionStatusReady))
	}
	if len(requested) == 0 {
		result := make([]string, 0, len(regions))
		for _, region := range regions {
			if ready[region.Name] {
				result = append(result, region.Name)
			}
		}
		sort.Strings(result)
		if len(result) == 0 {
			return nil, fmt.Errorf("the tenancy has no READY region subscriptions")
		}
		return result, nil
	}

	result := make([]string, 0, len(requested))
	for _, region := range requested {
		isReady, exists := ready[region]
		switch {
		case !exists:
			return nil, fmt.Errorf("requested region %s is not subscribed by this tenancy", region)
		case !isReady:
			return nil, fmt.Errorf("requested region %s is subscribed but not READY", region)
		default:
			result = append(result, region)
		}
	}
	return result, nil
}

func (c *Collector) searchAllRegions(
	ctx context.Context,
	regions []string,
) ([]resourceRef, []model.CollectionError) {
	type result struct {
		region string
		refs   []resourceRef
		err    error
	}

	jobs := make(chan string)
	results := make(chan result, len(regions))
	workers := min(c.cfg.Workers, len(regions))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for region := range jobs {
				refs, err := c.searchRegion(ctx, region)
				results <- result{region: region, refs: refs, err: err}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, region := range regions {
			select {
			case jobs <- region:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	deduplicated := map[string]resourceRef{}
	var errors []model.CollectionError
	for item := range results {
		for _, ref := range item.refs {
			deduplicated[ref.Region+"\x00"+ref.ID] = ref
		}
		if item.err != nil {
			errors = append(errors, model.CollectionError{
				Stage:   "search",
				Region:  item.region,
				Message: item.err.Error(),
			})
			fmt.Fprintf(c.logOutput, "warning: search failed in %s: %v\n", item.region, item.err)
		}
	}

	refs := make([]resourceRef, 0, len(deduplicated))
	for _, ref := range deduplicated {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Region != refs[j].Region {
			return refs[i].Region < refs[j].Region
		}
		return refs[i].ID < refs[j].ID
	})
	return refs, errors
}

func (c *Collector) searchRegion(ctx context.Context, region string) ([]resourceRef, error) {
	client, err := resourcesearch.NewResourceSearchClientWithConfigurationProvider(c.provider)
	if err != nil {
		return nil, fmt.Errorf("create Resource Search client: %w", err)
	}
	client.SetRegion(region)

	var refs []resourceRef
	var page *string
	for {
		response, searchErr := client.SearchResources(ctx, resourcesearch.SearchResourcesRequest{
			SearchDetails: resourcesearch.StructuredSearchDetails{
				Query: common.String(model.SearchQuery),
			},
			TenantId:        common.String(c.tenancyID),
			Limit:           common.Int(1000),
			Page:            page,
			RequestMetadata: retryMetadata(),
		})
		if searchErr != nil {
			return refs, searchErr
		}
		for _, item := range response.Items {
			if !strings.EqualFold(value(item.ResourceType), "autonomousdatabase") {
				continue
			}
			id := value(item.Identifier)
			if id != "" {
				refs = append(refs, resourceRef{Region: region, ID: id})
			}
		}
		if response.OpcNextPage == nil || value(response.OpcNextPage) == "" {
			break
		}
		page = response.OpcNextPage
	}
	return refs, nil
}

func (c *Collector) getAllDatabases(
	ctx context.Context,
	refs []resourceRef,
) ([]model.DatabaseRecord, []model.CollectionError) {
	type result struct {
		ref    resourceRef
		record model.DatabaseRecord
		err    error
	}

	jobs := make(chan resourceRef)
	results := make(chan result, min(len(refs), c.cfg.Workers))
	workers := min(c.cfg.Workers, len(refs))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ref := range jobs {
				record, err := c.getDatabase(ctx, ref)
				results <- result{ref: ref, record: record, err: err}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, ref := range refs {
			select {
			case jobs <- ref:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	records := make([]model.DatabaseRecord, 0, len(refs))
	var errors []model.CollectionError
	for item := range results {
		if item.err != nil {
			errors = append(errors, model.CollectionError{
				Stage:      "get_autonomous_database",
				Region:     item.ref.Region,
				ResourceID: item.ref.ID,
				Message:    item.err.Error(),
			})
			fmt.Fprintf(c.logOutput, "warning: database lookup failed in %s for %s: %v\n",
				item.ref.Region, item.ref.ID, item.err)
			continue
		}
		records = append(records, item.record)
	}
	return records, errors
}

func (c *Collector) getDatabase(
	ctx context.Context,
	ref resourceRef,
) (model.DatabaseRecord, error) {
	client, err := c.clientCache.client(ref.Region)
	if err != nil {
		return model.DatabaseRecord{}, err
	}
	response, err := client.GetAutonomousDatabase(ctx, database.GetAutonomousDatabaseRequest{
		AutonomousDatabaseId: common.String(ref.ID),
		RequestMetadata:      retryMetadata(),
	})
	if err != nil {
		return model.DatabaseRecord{}, err
	}
	adb := response.AutonomousDatabase
	return model.DatabaseRecord{
		Summary:       model.NewSummary(ref.Region, adb),
		Configuration: adb,
	}, nil
}

func retryMetadata() common.RequestMetadata {
	policy := common.DefaultRetryPolicy()
	return common.RequestMetadata{RetryPolicy: &policy}
}

func value(input *string) string {
	if input == nil {
		return ""
	}
	return *input
}

func boolValue(input *bool) bool {
	return input != nil && *input
}

type databaseClientCache struct {
	provider common.ConfigurationProvider
	mu       sync.Mutex
	clients  map[string]*database.DatabaseClient
}

func newDatabaseClientCache(provider common.ConfigurationProvider) *databaseClientCache {
	return &databaseClientCache{
		provider: provider,
		clients:  map[string]*database.DatabaseClient{},
	}
}

func (c *databaseClientCache) client(region string) (*database.DatabaseClient, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if client := c.clients[region]; client != nil {
		return client, nil
	}
	client, err := database.NewDatabaseClientWithConfigurationProvider(c.provider)
	if err != nil {
		return nil, fmt.Errorf("create Database client for %s: %w", region, err)
	}
	client.SetRegion(region)
	c.clients[region] = &client
	return &client, nil
}
