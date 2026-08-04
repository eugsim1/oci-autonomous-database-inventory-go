package oci

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/eugsim1/oci-autonomous-database-inventory-go/internal/config"
	"github.com/eugsim1/oci-autonomous-database-inventory-go/internal/model"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/database"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/oracle/oci-go-sdk/v65/resourcesearch"
)

type Collector struct {
	cfg             config.Config
	provider        common.ConfigurationProvider
	tenancyID       string
	logOutput       io.Writer
	databaseClients *databaseClientCache
	computeClients  *computeClientCache
	blockClients    *blockstorageClientCache
}

type resourceRef struct {
	Region         string
	ID             string
	CompartmentID  string
	DisplayName    string
	LifecycleState string
	TimeCreated    string
}

func NewCollector(cfg config.Config, logOutput io.Writer) (*Collector, error) {
	provider, tenancyID, err := configurationProvider(cfg)
	if err != nil {
		return nil, err
	}
	return &Collector{
		cfg:             cfg,
		provider:        provider,
		tenancyID:       tenancyID,
		logOutput:       logOutput,
		databaseClients: newDatabaseClientCache(provider),
		computeClients:  newComputeClientCache(provider),
		blockClients:    newBlockstorageClientCache(provider),
	}, nil
}

func (c *Collector) Collect(ctx context.Context) (model.Report, error) {
	report := model.Report{
		SchemaVersion:  "2.2",
		GeneratedAt:    time.Now().UTC(),
		TenancyOCID:    c.tenancyID,
		Authentication: c.cfg.AuthMode,
		SearchQueries: map[string]string{
			"autonomous_databases": model.AutonomousDatabaseSearchQuery,
			"compute_instances":    model.ComputeInstanceSearchQuery,
		},
		Databases:        []model.DatabaseRecord{},
		ComputeInstances: []model.ComputeInstanceRecord{},
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

	refs, searchErrors := c.searchAllRegions(
		ctx,
		scannedRegions,
		model.AutonomousDatabaseSearchQuery,
		"autonomousdatabase",
		"search_autonomous_database",
	)
	report.Errors = append(report.Errors, searchErrors...)
	fmt.Fprintf(c.logOutput, "OCI Search discovered %d Autonomous Database resource(s)\n", len(refs))

	records, detailErrors := c.getAllDatabases(ctx, refs, report.GeneratedAt)
	report.Databases = append(report.Databases, records...)
	report.Errors = append(report.Errors, detailErrors...)

	instanceRefs, instanceSearchErrors := c.searchAllRegions(
		ctx,
		scannedRegions,
		model.ComputeInstanceSearchQuery,
		"instance",
		"search_compute_instance",
	)
	report.Errors = append(report.Errors, instanceSearchErrors...)
	fmt.Fprintf(c.logOutput, "OCI Search discovered %d Compute instance resource(s)\n", len(instanceRefs))

	instances, instanceErrors := c.getAllComputeInstances(ctx, instanceRefs, report.GeneratedAt)
	report.ComputeInstances = append(report.ComputeInstances, instances...)
	report.Errors = append(report.Errors, instanceErrors...)
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
	query string,
	resourceType string,
	errorStage string,
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
				refs, err := c.searchRegion(ctx, region, query, resourceType)
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
			errors = append(errors, collectionError(errorStage, item.region, "", item.err))
			fmt.Fprintf(c.logOutput, "warning: %s failed in %s: %v\n", errorStage, item.region, item.err)
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

func (c *Collector) searchRegion(
	ctx context.Context,
	region string,
	query string,
	resourceType string,
) ([]resourceRef, error) {
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
				Query: common.String(query),
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
			if !strings.EqualFold(value(item.ResourceType), resourceType) {
				continue
			}
			id := value(item.Identifier)
			if id != "" {
				ref := resourceRef{
					Region:         region,
					ID:             id,
					CompartmentID:  value(item.CompartmentId),
					DisplayName:    value(item.DisplayName),
					LifecycleState: value(item.LifecycleState),
				}
				if item.TimeCreated != nil {
					ref.TimeCreated = item.TimeCreated.UTC().Format(time.RFC3339)
				}
				refs = append(refs, ref)
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
	asOf time.Time,
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
				record, err := c.getDatabase(ctx, ref, asOf)
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
			failure := collectionErrorForRef("get_autonomous_database", item.ref, item.err)
			errors = append(errors, failure)
			logCollectionWarning(c.logOutput, "database lookup", failure)
			continue
		}
		records = append(records, item.record)
	}
	return records, errors
}

func (c *Collector) getDatabase(
	ctx context.Context,
	ref resourceRef,
	asOf time.Time,
) (model.DatabaseRecord, error) {
	client, err := c.databaseClients.client(ref.Region)
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
		Summary:       model.NewSummaryAt(ref.Region, adb, asOf),
		Configuration: adb,
	}, nil
}

func (c *Collector) getAllComputeInstances(
	ctx context.Context,
	refs []resourceRef,
	asOf time.Time,
) ([]model.ComputeInstanceRecord, []model.CollectionError) {
	type result struct {
		ref    resourceRef
		record model.ComputeInstanceRecord
		errors []model.CollectionError
		fatal  error
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
				record, itemErrors, err := c.getComputeInstance(ctx, ref, asOf)
				results <- result{ref: ref, record: record, errors: itemErrors, fatal: err}
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

	records := make([]model.ComputeInstanceRecord, 0, len(refs))
	var errors []model.CollectionError
	for item := range results {
		if item.fatal != nil {
			failure := collectionErrorForRef("get_compute_instance", item.ref, item.fatal)
			errors = append(errors, failure)
			logCollectionWarning(c.logOutput, "instance lookup", failure)
			continue
		}
		records = append(records, item.record)
		errors = append(errors, item.errors...)
	}
	return records, errors
}

func (c *Collector) getComputeInstance(
	ctx context.Context,
	ref resourceRef,
	asOf time.Time,
) (model.ComputeInstanceRecord, []model.CollectionError, error) {
	client, err := c.computeClients.client(ref.Region)
	if err != nil {
		return model.ComputeInstanceRecord{}, nil, err
	}
	response, err := client.GetInstance(ctx, core.GetInstanceRequest{
		InstanceId:      common.String(ref.ID),
		RequestMetadata: retryMetadata(),
	})
	if err != nil {
		return model.ComputeInstanceRecord{}, nil, err
	}
	instance := response.Instance

	bootVolumes, bootComplete, bootErrors := c.getBootVolumes(ctx, ref.Region, instance, asOf)
	blockVolumes, blockComplete, blockErrors := c.getBlockVolumes(ctx, ref.Region, instance, asOf)
	itemErrors := append(bootErrors, blockErrors...)

	return model.NewComputeInstanceRecord(
		ref.Region,
		instance,
		bootVolumes,
		blockVolumes,
		bootComplete,
		blockComplete,
		asOf,
	), itemErrors, nil
}

func (c *Collector) getBootVolumes(
	ctx context.Context,
	region string,
	instance core.Instance,
	asOf time.Time,
) ([]model.BootVolumeRecord, bool, []model.CollectionError) {
	computeClient, err := c.computeClients.client(region)
	if err != nil {
		return nil, false, []model.CollectionError{collectionError(
			"list_boot_volume_attachments", region, value(instance.Id), err,
		)}
	}
	blockClient, err := c.blockClients.client(region)
	if err != nil {
		return nil, false, []model.CollectionError{collectionError(
			"get_boot_volume", region, value(instance.Id), err,
		)}
	}

	var attachments []core.BootVolumeAttachment
	var page *string
	for {
		response, listErr := computeClient.ListBootVolumeAttachments(
			ctx,
			core.ListBootVolumeAttachmentsRequest{
				AvailabilityDomain: instance.AvailabilityDomain,
				CompartmentId:      instance.CompartmentId,
				InstanceId:         instance.Id,
				Limit:              common.Int(1000),
				Page:               page,
				RequestMetadata:    retryMetadata(),
			},
		)
		if listErr != nil {
			return nil, false, []model.CollectionError{collectionError(
				"list_boot_volume_attachments", region, value(instance.Id), listErr,
			)}
		}
		for _, attachment := range response.Items {
			if attachment.LifecycleState == core.BootVolumeAttachmentLifecycleStateAttached {
				attachments = append(attachments, attachment)
			}
		}
		if response.OpcNextPage == nil || value(response.OpcNextPage) == "" {
			break
		}
		page = response.OpcNextPage
	}

	records := make([]model.BootVolumeRecord, 0, len(attachments))
	var errors []model.CollectionError
	for _, attachment := range attachments {
		volumeID := value(attachment.BootVolumeId)
		response, getErr := blockClient.GetBootVolume(ctx, core.GetBootVolumeRequest{
			BootVolumeId:    attachment.BootVolumeId,
			RequestMetadata: retryMetadata(),
		})
		if getErr != nil {
			records = append(records, model.NewUnavailableBootVolumeRecord(attachment))
			errors = append(errors, collectionError("get_boot_volume", region, volumeID, getErr))
			continue
		}
		records = append(records, model.NewBootVolumeRecord(
			attachment,
			response.BootVolume,
			asOf,
		))
	}
	return records, true, errors
}

func (c *Collector) getBlockVolumes(
	ctx context.Context,
	region string,
	instance core.Instance,
	asOf time.Time,
) ([]model.BlockVolumeRecord, bool, []model.CollectionError) {
	computeClient, err := c.computeClients.client(region)
	if err != nil {
		return nil, false, []model.CollectionError{collectionError(
			"list_block_volume_attachments", region, value(instance.Id), err,
		)}
	}
	blockClient, err := c.blockClients.client(region)
	if err != nil {
		return nil, false, []model.CollectionError{collectionError(
			"get_block_volume", region, value(instance.Id), err,
		)}
	}

	var attachments []core.VolumeAttachment
	var page *string
	for {
		response, listErr := computeClient.ListVolumeAttachments(ctx, core.ListVolumeAttachmentsRequest{
			CompartmentId:      instance.CompartmentId,
			AvailabilityDomain: instance.AvailabilityDomain,
			InstanceId:         instance.Id,
			Limit:              common.Int(1000),
			Page:               page,
			RequestMetadata:    retryMetadata(),
		})
		if listErr != nil {
			return nil, false, []model.CollectionError{collectionError(
				"list_block_volume_attachments", region, value(instance.Id), listErr,
			)}
		}
		for _, attachment := range response.Items {
			if attachment.GetLifecycleState() == core.VolumeAttachmentLifecycleStateAttached {
				attachments = append(attachments, attachment)
			}
		}
		if response.OpcNextPage == nil || value(response.OpcNextPage) == "" {
			break
		}
		page = response.OpcNextPage
	}

	records := make([]model.BlockVolumeRecord, 0, len(attachments))
	var errors []model.CollectionError
	for _, attachment := range attachments {
		volumeID := value(attachment.GetVolumeId())
		response, getErr := blockClient.GetVolume(ctx, core.GetVolumeRequest{
			VolumeId:        attachment.GetVolumeId(),
			RequestMetadata: retryMetadata(),
		})
		if getErr != nil {
			records = append(records, model.NewUnavailableBlockVolumeRecord(attachment))
			errors = append(errors, collectionError("get_block_volume", region, volumeID, getErr))
			continue
		}
		records = append(records, model.NewBlockVolumeRecord(
			attachment,
			response.Volume,
			asOf,
		))
	}
	return records, true, errors
}

func collectionError(stage, region, resourceID string, err error) model.CollectionError {
	item := model.CollectionError{
		Stage:      stage,
		Region:     region,
		ResourceID: resourceID,
		Message:    err.Error(),
	}

	var serviceError common.ServiceError
	if errors.As(err, &serviceError) {
		item.HTTPStatusCode = serviceError.GetHTTPStatusCode()
		item.ServiceCode = serviceError.GetCode()
		item.OPCRequestID = serviceError.GetOpcRequestID()
		retryable := common.IsErrorRetryableByDefault(err)
		item.Retryable = &retryable
		item.Diagnosis, item.SuggestedActions = diagnoseServiceError(stage, item)
	}

	var richError common.ServiceErrorRichInfo
	if errors.As(err, &richError) {
		item.TargetService = richError.GetTargetService()
		item.OperationName = richError.GetOperationName()
		item.RequestTimestamp = richError.GetTimestamp().UTC().Format(time.RFC3339)
		item.RequestEndpoint = richError.GetRequestTarget()
		item.ClientVersion = richError.GetClientVersion()
		item.TroubleshootingLink = richError.GetErrorTroubleshootingLink()
		item.OperationReferenceLink = richError.GetOperationReferenceLink()
	}
	return item
}

func collectionErrorForRef(stage string, ref resourceRef, err error) model.CollectionError {
	item := collectionError(stage, ref.Region, ref.ID, err)
	item.SearchCompartmentID = ref.CompartmentID
	item.SearchDisplayName = ref.DisplayName
	item.SearchLifecycleState = ref.LifecycleState
	item.SearchTimeCreated = ref.TimeCreated
	return item
}

func diagnoseServiceError(stage string, item model.CollectionError) (string, []string) {
	if item.HTTPStatusCode != 404 || !strings.EqualFold(item.ServiceCode, "NotAuthorizedOrNotFound") {
		return "", nil
	}

	diagnosis := "OCI uses this 404 when the resource does not exist or the caller is not authorized to access it. The default OCI retry policy classifies it as non-retryable, so repeating the unchanged request is not expected to resolve it."
	actions := []string{
		"Verify that the resource OCID still exists, is in this tenancy and region, and has not been terminated or moved.",
		"Verify the signing user, instance principal, or resource principal and its effective policies in the resource compartment.",
		"If --tenancy-id was supplied, verify that it is the tenancy represented by the authentication provider.",
		"Retry after allowing for OCI Search index consistency; if the direct Get still fails, retain the OPC request ID for Oracle Support.",
	}
	if stage == "get_autonomous_database" {
		diagnosis += " GetAutonomousDatabase and visibility of autonomous-database results in OCI Search both require AUTONOMOUS_DATABASE_INSPECT. If Search found the OCID but Get immediately returned this response, a stale Search entry after deletion or movement is a leading explanation; a changed, compartment-scoped, or conditional IAM grant remains possible."
		actions[1] = "Verify that the principal has `inspect autonomous-databases` in the database compartment (or tenancy), including any identity-domain, dynamic-group, tag, and policy conditions."
	}
	return diagnosis, actions
}

func logCollectionWarning(output io.Writer, operation string, item model.CollectionError) {
	if item.HTTPStatusCode == 0 {
		fmt.Fprintf(output, "warning: %s failed in %s for %s: %s\n",
			operation, item.Region, item.ResourceID, item.Message)
		return
	}
	retryable := "unknown"
	if item.Retryable != nil {
		retryable = fmt.Sprintf("%t", *item.Retryable)
	}
	fmt.Fprintf(output,
		"warning: %s failed in %s for %s: HTTP %d %s; retryable=%s; opc-request-id=%s; full diagnostics are in the failed-requests report\n",
		operation,
		item.Region,
		item.ResourceID,
		item.HTTPStatusCode,
		item.ServiceCode,
		retryable,
		item.OPCRequestID,
	)
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

type computeClientCache struct {
	provider common.ConfigurationProvider
	mu       sync.Mutex
	clients  map[string]*core.ComputeClient
}

func newComputeClientCache(provider common.ConfigurationProvider) *computeClientCache {
	return &computeClientCache{
		provider: provider,
		clients:  map[string]*core.ComputeClient{},
	}
}

func (c *computeClientCache) client(region string) (*core.ComputeClient, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if client := c.clients[region]; client != nil {
		return client, nil
	}
	client, err := core.NewComputeClientWithConfigurationProvider(c.provider)
	if err != nil {
		return nil, fmt.Errorf("create Compute client for %s: %w", region, err)
	}
	client.SetRegion(region)
	c.clients[region] = &client
	return &client, nil
}

type blockstorageClientCache struct {
	provider common.ConfigurationProvider
	mu       sync.Mutex
	clients  map[string]*core.BlockstorageClient
}

func newBlockstorageClientCache(provider common.ConfigurationProvider) *blockstorageClientCache {
	return &blockstorageClientCache{
		provider: provider,
		clients:  map[string]*core.BlockstorageClient{},
	}
}

func (c *blockstorageClientCache) client(region string) (*core.BlockstorageClient, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if client := c.clients[region]; client != nil {
		return client, nil
	}
	client, err := core.NewBlockstorageClientWithConfigurationProvider(c.provider)
	if err != nil {
		return nil, fmt.Errorf("create Block Storage client for %s: %w", region, err)
	}
	client.SetRegion(region)
	c.clients[region] = &client
	return &client, nil
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
