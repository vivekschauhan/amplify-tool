package metric

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/Axway/agent-sdk/pkg/api"
	"github.com/Axway/agent-sdk/pkg/apic/auth"
	"github.com/Axway/agent-sdk/pkg/config"
	"github.com/Axway/agent-sdk/pkg/transaction/metric"

	utillog "github.com/Axway/agent-sdk/pkg/util/log"
	"github.com/vivekschauhan/amplify-tool/pkg/log"
	"github.com/vivekschauhan/amplify-tool/pkg/tools"
)

type Tool interface {
	Run() error
}

type tool struct {
	apiClient   api.Client
	cfg         *Config
	logger      *logrus.Logger
	tokenGetter auth.PlatformTokenGetter
	cacheData   *data
	batchSize   int
	reporter    *metric.Reporter
}

func NewTool(cfg *Config) Tool {
	logger := log.GetLogger(cfg.Level, cfg.Format)
	_, tokenGetter := tools.CreateAPICClient(&cfg.Config)
	utillog.GlobalLoggerConfig.Level(cfg.Level).
		Format(cfg.Format).
		Apply()
	return &tool{
		logger:      logger,
		cfg:         cfg,
		apiClient:   api.NewClient(config.NewTLSConfig(), "", api.WithSingleURL()),
		tokenGetter: tokenGetter,
		cacheData:   &data{},
		batchSize:   cfg.BatchSize,
		reporter: &metric.Reporter{
			AgentName:       cfg.AgentName,
			AgentVersion:    cfg.AgentVersion,
			AgentSDKVersion: cfg.AgentSDKVersion,
			AgentType:       cfg.AgentType,
		},
	}
}

func (t *tool) Run() error {
	t.logger.Info("Amplify Cached Metric Upload Tool")

	if t.cfg.SkipUsageUpload && (t.cfg.UsageProduct == "" || t.cfg.EnvironmentID == "") {
		t.logger.Error("A a usage product and environment id must be set to upload usage")
		return nil
	}

	// read in metric file
	t.readCacheFile()

	if !t.cfg.SkipUsageUpload {
		t.uploadUsage()
	}

	if !t.cfg.SkipMetricUpload {
		t.uploadMetrics()
	}

	return nil
}

func (t *tool) readCacheFile() {
	logger := t.logger.WithField("filename", t.cfg.MetricCacheFile)
	buf, err := os.ReadFile(t.cfg.MetricCacheFile)
	if err != nil {
		logger.WithError(err).Error("unable to read metric cache file")
		return
	}
	err = json.Unmarshal(buf, t.cacheData)
	if err != nil {
		logger.WithError(err).Error("could not load metric cache file")
		return
	}
	logger.WithField("items", len(t.cacheData.Cache)).Info("read metric cache file")
}

func (t *tool) uploadUsage() {
	logger := t.logger.WithField("action", "usage")
	logger.Info("starting to upload usage")

	usageItem, ok := t.cacheData.Cache[usageKey]
	if !ok {
		logger.Error("could not find usage data in metric cache")
		return
	}
	// read usage count
	count, ok := usageItem.Object.(float64)
	if !ok {
		logger.Error("could not read usage count from metric data")
		return
	}
	endTime := time.Unix(usageItem.UpdateTime, 0)
	logger = logger.WithField("count", count).WithField("endTime", endTime)
	logger.Debug("reading usage start time")

	// read usage start time
	usageTimeItem, ok := t.cacheData.Cache[usageStartKey]
	if !ok {
		logger.Error("could not find usage start time in metric cache")
		return
	}
	// read usage count
	startTimeStr, ok := usageTimeItem.Object.(string)
	if !ok {
		logger.Error("could not read usage start time from metric data")
		return
	}
	startTime, _ := time.Parse(time.RFC3339Nano, startTimeStr)
	logger = logger.WithField("startTime", startTime)
	logger.Info("create and upload usage event")

	token, _ := t.tokenGetter.GetToken()
	schema, _ := url.JoinPath(t.cfg.PlatformURL, schemaPath)

	// create usage event
	usageEvent := metric.UsageEvent{
		OrgGUID:     getOrgGUID(token),
		EnvID:       t.cfg.EnvironmentID,
		Timestamp:   metric.ISO8601Time(startTime),
		Granularity: int(endTime.Sub(startTime).Milliseconds()),
		SchemaID:    schema,
		Report: map[string]metric.UsageReport{
			startTime.Format(reportKeyFormat): metric.UsageReport{
				Product: t.cfg.UsageProduct,
				Meta:    map[string]interface{}{},
				Usage: map[string]int64{
					fmt.Sprintf("%s.Transactions", t.cfg.UsageProduct): int64(count),
				},
			},
		},
		Meta: t.createUsageMetaData(),
	}

	if t.cfg.DryRun {
		data, _ := json.Marshal(usageEvent)
		logger.WithField("usageEvent", string(data)).Info("would upload usage data")
		return
	}

	data, contentType, err := createMultipartFormData(usageEvent)
	if err != nil {
		logger.WithError(err).Error("creating multipart usage event")
		return
	}

	headers := map[string]string{
		"Content-Type":  contentType,
		"Authorization": "Bearer " + token,
	}

	request := api.Request{
		Method:  api.POST,
		URL:     t.cfg.PlatformURL + "/api/v1/usage",
		Headers: headers,
		Body:    data.Bytes(),
	}

	response, err := t.apiClient.Send(request)
	if err != nil {
		logger.WithError(err).Error("publishing usage")
		return
	}

	logger.WithField("statusCode", response.Code)
	if response.Code != 202 {
		logger.WithField("body", string(response.Body)).Error("unexpected response uploading data")
		return
	}

	logger.Debug("successfully uploaded usage")
}

func (t *tool) createUsageMetaData() map[string]interface{} {
	meta := map[string]interface{}{}
	if t.reporter.AgentName != "" {
		meta["AgentName"] = t.reporter.AgentName
	}
	if t.reporter.AgentSDKVersion != "" {
		meta["AgentSDKVersion"] = t.reporter.AgentSDKVersion
	}
	if t.reporter.AgentType != "" {
		meta["AgentType"] = t.reporter.AgentType
	}
	if t.reporter.AgentVersion != "" {
		meta["AgentVersion"] = t.reporter.AgentVersion
	}
	return meta
}

func (t *tool) uploadMetrics() {
	logger := t.logger.WithField("action", "metrics")
	logger.Info("starting to upload metrics")

	// read metric start time
	metricTimeItem, ok := t.cacheData.Cache[metricStartKey]
	if !ok {
		logger.Error("could not find metric start time in metric cache")
		return
	}
	// read metric start time
	startTimeStr, ok := metricTimeItem.Object.(string)
	if !ok {
		logger.Error("could not get metric start time from metric data")
		return
	}
	startTime, err := time.Parse(reportKeyFormat, startTimeStr)
	if err != nil {
		logger.WithError(err).Error("could not read metric start time from metric data")
		return
	}
	logger = logger.WithField("startTime", startTime)

	// read last update time as end time
	endTime := time.Unix(metricTimeItem.UpdateTime, 0)
	logger = logger.WithField("endTime", endTime)

	batch := []publishMetric{}
	for key, item := range t.cacheData.Cache {
		if !strings.HasPrefix(key, metricPrefix) {
			// skip non  metric keys
			continue
		}
		uuid := uuid.NewString()
		keyLogger := logger.WithField("metricKey", key).WithField("eventID", uuid)

		// convert the object to json
		jsonData, err := json.Marshal(item.Object)
		if err != nil {
			keyLogger.WithError(err).Error("could not get metric data")
			continue
		}

		metricData := cachedMetric{}
		// read teh data back into the expected object
		err = json.Unmarshal(jsonData, &metricData)
		if err != nil {
			keyLogger.WithError(err).Error("could not get metric data")
			continue
		}

		keyLogger.Info("creating metric and adding to batch")
		batch = append(batch, publishMetric{
			Subscription:  metricData.Subscription,
			App:           metricData.App,
			Product:       metricData.Product,
			API:           metricData.API,
			AssetResource: metricData.AssetResource,
			ProductPlan:   metricData.ProductPlan,
			Unit: units{
				Transactions: transaction{
					Count:    int(metricData.Count),
					Status:   metricData.StatusCode,
					Quota:    metricData.Quota,
					Response: t.getResponseData(metricData.Values),
				},
			},
			Reporter: &metric.Reporter{
				AgentName:        t.reporter.AgentName,
				AgentVersion:     t.reporter.AgentVersion,
				AgentType:        t.reporter.AgentType,
				AgentSDKVersion:  t.reporter.AgentSDKVersion,
				ObservationDelta: int64(endTime.Sub(startTime).Milliseconds()),
			},
			eventID:   uuid,
			startTime: startTime,
			eventType: metricEvent,
		})
		if len(batch) == t.batchSize {
			t.sendMetricBatch(logger, batch, startTime)
			batch = []publishMetric{}
		}
	}
	t.sendMetricBatch(logger, batch, startTime) // send final batch
}

func (t *tool) getResponseData(values []int64) responseData {
	count := len(values)
	min := int64(-1)
	max := int64(-1)
	total := int64(0)

	for _, v := range values {
		if min == -1 || v < min {
			min = v
		}
		if max == -1 || v > max {
			max = v
		}
		total += v
	}

	return responseData{
		Min: min,
		Max: max,
		Avg: float64(total) / float64(count),
	}
}

func (t *tool) sendMetricBatch(logger *logrus.Entry, batch []publishMetric, startTime time.Time) {
	logger = logger.WithField("batchSize", len(batch)).WithField("batchID", uuid.NewString())
	if len(batch) == 0 {
		logger.Debug("no event in batch")
		return
	}
	token, _ := t.tokenGetter.GetToken()

	jsonData, err := json.Marshal(t.createV4Events(batch))
	if err != nil {
		logger.WithError(err).Error("creating json from event batch")
		return
	}

	if t.cfg.DryRun {
		logger.WithField("batch", string(jsonData)).Info("would compress and send")
		return
	}

	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	_, err = gz.Write(jsonData)
	if err != nil {
		logger.WithError(err).Error("compressing event data")
		return
	}
	err = gz.Close()
	if err != nil {
		logger.WithError(err).Error("compressing event data")
		return
	}

	req := api.Request{
		Method: http.MethodPost,
		URL:    "https://" + t.cfg.TraceabilityHost,
		Headers: map[string]string{
			"Authorization":     "Bearer " + token,
			"Capture-Org-ID":    t.cfg.OrgID,
			"User-Agent":        "generic-service",
			"Content-Type":      "application/json; charset=UTF-8",
			"Content-Encoding":  "gzip",
			"Axway-Target-Flow": metricFlow,
			"Timestamp":         strconv.FormatInt(startTime.UTC().UnixMilli(), 10),
		},
		Body: b.Bytes(),
	}

	resp, err := t.apiClient.Send(req)
	if err != nil {
		logger.WithError(err).Error("sending event data")
		return
	}

	logger = logger.WithField("statusCode", resp.Code)
	if resp.Code != http.StatusOK {
		logger.WithError(err).Error("unexpected status code")
		return
	}

	logger.Info("data sent successfully")
}

func (t *tool) createV4Events(metricData []publishMetric) []metric.V4Event {
	token, _ := t.tokenGetter.GetToken()
	events := []metric.V4Event{}
	for _, e := range metricData {
		events = append(events,
			metric.V4Event{
				ID:        uuid.NewString(),
				Timestamp: e.startTime.UnixMilli(),
				Event:     metricEvent,
				App:       getOrgGUID(token),
				Version:   "4",
				Distribution: &metric.V4EventDistribution{
					Environment: t.cfg.EnvironmentID,
					Version:     "1",
				},
				Data: e,
			})
	}
	return events
}
