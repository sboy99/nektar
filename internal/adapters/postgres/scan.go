package postgres

import (
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"
)

func float32SliceToArray(v []float32) *pgtype.FlatArray[float32] {
	if v == nil {
		return nil
	}
	arr := pgtype.FlatArray[float32](v)
	return &arr
}

func arrayToFloat32Slice(arr pgtype.FlatArray[float32]) []float32 {
	if len(arr) == 0 {
		return nil
	}
	return []float32(arr)
}

func stringSliceToArray(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func arrayToStringSlice(v []string) []string {
	if v == nil {
		return nil
	}
	return v
}

func headersToJSONB(headers map[string]string) ([]byte, error) {
	if headers == nil {
		headers = map[string]string{}
	}
	return json.Marshal(headers)
}

func jsonbToHeaders(data []byte) (map[string]string, error) {
	if len(data) == 0 {
		return map[string]string{}, nil
	}
	var headers map[string]string
	if err := json.Unmarshal(data, &headers); err != nil {
		return nil, err
	}
	if headers == nil {
		headers = map[string]string{}
	}
	return headers, nil
}
