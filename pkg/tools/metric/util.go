package metric

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"

	"github.com/Axway/agent-sdk/pkg/transaction/metric"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func getOrgGUID(authToken string) string {
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	claims := jwt.MapClaims{}
	_, _, err := parser.ParseUnverified(authToken, claims)
	if err != nil {
		return ""
	}

	claim, ok := claims["org_guid"]
	if ok {
		return claim.(string)
	}
	return ""
}

func createMultipartFormData(event metric.UsageEvent) (b bytes.Buffer, contentType string, err error) {
	buffer, _ := json.Marshal(event)
	w := multipart.NewWriter(&b)
	defer w.Close()
	w.WriteField("organizationId", event.OrgGUID)

	var fw io.Writer
	if fw, err = createFilePart(w, uuid.New().String()+".json"); err != nil {
		return
	}
	if _, err = io.Copy(fw, bytes.NewReader(buffer)); err != nil {
		return
	}
	contentType = w.FormDataContentType()

	return
}

// createFilePart - adds the file part to the request
func createFilePart(w *multipart.Writer, filename string) (io.Writer, error) {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	h.Set("Content-Type", "application/json")
	return w.CreatePart(h)
}
