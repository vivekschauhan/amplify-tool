package metric

import (
	"time"

	"github.com/Axway/agent-sdk/pkg/cache"
	"github.com/Axway/agent-sdk/pkg/transaction/models"
	"github.com/sirupsen/logrus"
)

const (
	metricStartKey  = "metric_start_time"
	usageKey        = "usage_count"
	usageStartKey   = "usage_start_time"
	metricPrefix    = "metric."
	schemaPath      = "/api/v1/report.schema.json"
	reportKeyFormat = "2006-01-02T15:04:05Z"
	metricEvent     = "api.transaction.status.metric"
	metricFlow      = "api-central-metric"
)

type data struct {
	Cache map[string]cache.Item `json:"cache"`
}

type cachedMetric struct {
	Subscription  *models.ResourceReference            `json:"subscription,omitempty"`
	App           *models.ApplicationResourceReference `json:"application,omitempty"`
	Product       *models.ProductResourceReference     `json:"product,omitempty"`
	API           *models.APIResourceReference         `json:"api,omitempty"`
	AssetResource *models.ResourceReference            `json:"assetResource,omitempty"`
	ProductPlan   *models.ResourceReference            `json:"productPlan,omitempty"`
	Quota         *models.ResourceReference            `json:"quota,omitempty"`
	Unit          *models.Unit                         `json:"unit,omitempty"`
	StatusCode    string                               `json:"statusCode,omitempty"`
	Count         int64                                `json:"count"`
	Values        []int64                              `json:"values,omitempty"`
	starTime      time.Time
	eventType     string
	eventID       string
}

type responseData struct {
	Min int64   `json:"min,omitempty"`
	Max int64   `json:"max,omitempty"`
	Avg float64 `json:"avg,omitempty"`
}

type transaction struct {
	Count    int                       `json:"count,omitempty"`
	Response responseData              `json:"response,omitempty"`
	Status   string                    `json:"status,omitempty"`
	Quota    *models.ResourceReference `json:"quota,omitempty"`
}

type units struct {
	Transactions transaction `json:"transactions,omitempty"`
}

type publishMetric struct {
	Subscription  *models.ResourceReference            `json:"subscription,omitempty"`
	App           *models.ApplicationResourceReference `json:"application,omitempty"`
	Product       *models.ProductResourceReference     `json:"product,omitempty"`
	API           *models.APIResourceReference         `json:"api,omitempty"`
	AssetResource *models.ResourceReference            `json:"assetResource,omitempty"`
	ProductPlan   *models.ResourceReference            `json:"productPlan,omitempty"`

	Unit      units `json:"units,omitempty"`
	starTime  time.Time
	eventType string
	eventID   string
}

func (c publishMetric) GetStartTime() time.Time {
	return c.starTime
}
func (c publishMetric) GetType() string {
	return c.eventType
}
func (c publishMetric) GetEventID() string {
	return c.eventID
}
func (c publishMetric) GetLogFields() logrus.Fields {
	return nil
}
