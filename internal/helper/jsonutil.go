package helper

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

// ParseJSON 去掉可能包裹的 Markdown ```json``` 块再解析
func ParseJSON(raw string, v interface{}) error {
	trim := strings.TrimSpace(raw)
	trim = strings.Trim(trim, "```")
	return json.Unmarshal([]byte(trim), v)
}

// ValidateJSONSchema 验证 bytes 是否符合 schema
func ValidateJSONSchema(schemaPath string, data []byte) error {
	absPath, err := filepath.Abs(schemaPath)
	if err != nil {
		return err
	}
	sl := gojsonschema.NewReferenceLoader("file://" + absPath)
	dl := gojsonschema.NewBytesLoader(data)
	res, err := gojsonschema.Validate(sl, dl)
	if err != nil {
		return err
	}
	if !res.Valid() {
		return fmt.Errorf("schema invalid: %v", res.Errors())
	}
	return nil
}

// LoadFile convenience
func LoadFile(path string) ([]byte, error) { return os.ReadFile(path) }
