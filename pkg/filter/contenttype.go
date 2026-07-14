package filter

import (
    "encoding/json"
    "fmt"
    "strings"

    "github.com/ffuf/ffuf/v2/pkg/ffuf"
)

type ContentTypeFilter struct {
    Value []string
}

func NewContentTypeFilter(value string) (ffuf.FilterProvider, error) {
    vals := make([]string, 0)
    for _, v := range strings.Split(value, ",") {
        v = strings.TrimSpace(v)
        if v == "" {
            continue
        }
        vals = append(vals, v)
    }
    if len(vals) == 0 {
        return &ContentTypeFilter{}, fmt.Errorf("Content-Type filter or matcher (-fct / -mct): invalid value %s", value)
    }
    return &ContentTypeFilter{Value: vals}, nil
}

func (f *ContentTypeFilter) MarshalJSON() ([]byte, error) {
    return json.Marshal(&struct{
        Value string `json:"value"`
    }{Value: strings.Join(f.Value, ",")})
}

func (f *ContentTypeFilter) Filter(response *ffuf.Response) (bool, error) {
    // Normalize response content type (strip params)
    respct := strings.ToLower(strings.TrimSpace(response.ContentType))
    if respct == "" {
        respct = ""
    }
    if strings.Contains(respct, ";") {
        respct = strings.TrimSpace(strings.SplitN(respct, ";", 2)[0])
    }
    for _, v := range f.Value {
        v = strings.ToLower(strings.TrimSpace(v))
        if v == "all" {
            return true, nil
        }
        if v == "" {
            continue
        }
        // support wildcard like application/*
        if strings.HasSuffix(v, "/*") {
            prefix := strings.SplitN(v, "/", 2)[0]
            if strings.HasPrefix(respct, prefix+"/") {
                return true, nil
            }
            continue
        }
        if strings.Contains(v, "/") {
            // exact mediatype match
            if respct == v {
                return true, nil
            }
            continue
        }
        // fallback: substring match (e.g., "json" matches "application/json")
        if strings.Contains(respct, v) {
            return true, nil
        }
    }
    return false, nil
}

func (f *ContentTypeFilter) Repr() string {
    return strings.Join(f.Value, ",")
}

func (f *ContentTypeFilter) ReprVerbose() string {
    return fmt.Sprintf("Response Content-Type: %s", f.Repr())
}
