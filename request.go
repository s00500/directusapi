package directusapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"reflect"
)

const tagName = "directus"

type request struct {
	ctx    context.Context
	method string
	url    string
	qv     map[string]string
	body   any
}

func (a *API[R, W, PK]) executeRequest(r request, expectedStatus int, dest any) error {
	if dest != nil && reflect.ValueOf(dest).Kind() != reflect.Ptr {
		return fmt.Errorf("dest has to be a pointer")
	}

	var b io.Reader
	if r.body != nil {
		// todo: custom json marshal based on custom struct tag with reflection
		bodyBytes, err := json.Marshal(r.body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		b = bytes.NewBuffer(bodyBytes)
	}

	req, err := http.NewRequestWithContext(
		r.ctx,
		r.method,
		r.url,
		b,
	)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	queryValues := url.Values{}

	for k, v := range r.qv {
		queryValues.Set(k, v)
	}

	req.URL.RawQuery = queryValues.Encode()

	req.Header.Set("Authorization", "Bearer "+a.BearerToken)
	req.Header.Set("Content-Type", "application/json")

	if a.debug {
		reqDump, _ := httputil.DumpRequestOut(req, true)
		fmt.Println("--- Request start ---")
		fmt.Println(string(reqDump))
		fmt.Println("--- Request end ---")
	}

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %v", err)
	}
	defer resp.Body.Close()

	if a.debug {
		respDump, _ := httputil.DumpResponse(resp, true)
		fmt.Println("--- Response start ---")
		fmt.Println(string(respDump))
		fmt.Println("--- Response end ---")
	}

	if resp.StatusCode != expectedStatus {
		respBytes, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %s: %s", resp.Status, string(respBytes))
	}

	if dest != nil {
		// todo: custom json unmarshal based on custom tag
		err = json.NewDecoder(resp.Body).Decode(dest)
		if err != nil {
			return fmt.Errorf("decoding json response: %w", err)
		}
	}

	return nil
}

func jsonMarshal(inp any) ([]byte, error) {
	jsonFieldsMap := mapByStructTag(inp)
	return json.Marshal(jsonFieldsMap)
}

func mapByStructTag(inp any) OrderedMap {
	output := OrderedMap{}
	structVal := reflect.ValueOf(inp)
	structType := structVal.Type()
	// iterate through struct fields
	for i := 0; i < structVal.NumField(); i++ {
		fieldVal := structVal.Field(i)
		fieldType := structType.Field(i)

		directusFieldName, ok := fieldType.Tag.Lookup(tagName)
		if !ok {
			// field has no tag, skipping the field
			continue
		}
		switch fieldVal.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			output = append(output, KeyVal{directusFieldName, fieldVal.Int()})
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			output = append(output, KeyVal{directusFieldName, fieldVal.Uint()})
		case reflect.Float32, reflect.Float64:
			output = append(output, KeyVal{directusFieldName, fieldVal.Float()})
		case reflect.Bool:
			output = append(output, KeyVal{directusFieldName, fieldVal.Bool()})
		case reflect.String:
			output = append(output, KeyVal{directusFieldName, fieldVal.String()})
		case reflect.Array, reflect.Struct, reflect.Slice, reflect.Map:
			panic("not implemented")
		default:
			panic("unsupported field type")
		}
	}
	return output
}
