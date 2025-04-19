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
		Meta: map[string]interface{}{},
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

func (t *tool) uploadMetrics() {
	logger := t.logger.WithField("action", "metrics")
	logger.Info("starting to upload metrics")

	// read metric start time
	metricTimeItem, ok := t.cacheData.Cache[metricStartKey]
	if !ok {
		logger.Error("could not find metric start time in metric cache")
		return
	}
	// read usage count
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

	for key, item := range t.cacheData.Cache {
		if !strings.HasPrefix(key, metricPrefix) {
			// skip non  metric keys
			continue
		}
		uuid := uuid.NewString()
		logger := logger.WithField("metricKey", key).WithField("eventID", uuid)

		// convert the object to json
		jsonData, err := json.Marshal(item.Object)
		if err != nil {
			logger.WithError(err).Error("could not get metric data")
			continue
		}

		metric := cachedMetric{}
		// read teh data back into the expected object
		err = json.Unmarshal(jsonData, &metric)
		if err != nil {
			logger.WithError(err).Error("could not get metric data")
			continue
		}
		metric.eventID = uuid
		metric.starTime = startTime
		metric.eventType = metricEvent

		logger.Info("creating and uploading metric")
		t.sendMetric(publishMetric{
			Subscription:  metric.Subscription,
			App:           metric.App,
			Product:       metric.Product,
			API:           metric.API,
			AssetResource: metric.AssetResource,
			ProductPlan:   metric.ProductPlan,
			Unit: units{
				Transactions: transaction{
					Count:    int(metric.Count),
					Status:   metric.StatusCode,
					Quota:    metric.Quota,
					Response: t.getResponseData(metric.Values),
				},
			},
			eventID:   uuid,
			starTime:  startTime,
			eventType: metricEvent,
		})
	}
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

func (t *tool) sendMetric(metric publishMetric) {
	token, _ := t.tokenGetter.GetToken()

	jsonData, err := json.Marshal(t.createV4Event(metric))
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return
	}

	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	gz.Write(jsonData)
	gz.Close()

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
			"Timestamp":         strconv.FormatInt(metric.starTime.UTC().UnixMilli(), 10),
		},
		Body: b.Bytes(),
	}
	// beat.Event{}

	resp, err := t.apiClient.Send(req)
	if err != nil {
		fmt.Println("Error sending data to Logstash:", err)
		return
	}

	if resp.Code != http.StatusOK {
		fmt.Println("Error: Logstash returned status code", resp.Code)
		return
	}

	fmt.Println("Data sent successfully to Logstash")
}

func (t *tool) createV4Event(metricData publishMetric) []metric.V4Event {
	return []metric.V4Event{
		{
			ID:        uuid.NewString(),
			Timestamp: metricData.starTime.UnixMilli(),
			Event:     metricEvent,
			App:       "7fd20fc9-ef33-4b19-981e-1afff6bc4c97", // TODO
			Version:   "4",
			Distribution: &metric.V4EventDistribution{
				Environment: t.cfg.EnvironmentID,
				Version:     "1",
			},
			Data: metricData,
		},
	}
}
