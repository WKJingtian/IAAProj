package config

import (
	"bytes"
	"encoding"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

type csvFieldBinding struct {
	index []int
	typ   reflect.Type
}

// CSVCellError describes a single cell conversion failure while loading CSV.
type CSVCellError struct {
	Row    int
	Column int
	Header string
	Value  string
	Err    error
}

func (e CSVCellError) Error() string {
	return fmt.Sprintf("row %d column %d (%q): %v", e.Row, e.Column, e.Header, e.Err)
}

func (e CSVCellError) Unwrap() error {
	return e.Err
}

// CSVLoadError aggregates non-fatal cell conversion failures.
type CSVLoadError struct {
	Path   string
	Issues []CSVCellError
}

func (e *CSVLoadError) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return ""
	}
	return fmt.Sprintf("load csv file %q encountered %d issue(s); first issue: %v", e.Path, len(e.Issues), e.Issues[0])
}

func (e *CSVLoadError) Unwrap() []error {
	if e == nil || len(e.Issues) == 0 {
		return nil
	}
	errs := make([]error, 0, len(e.Issues))
	for _, issue := range e.Issues {
		errs = append(errs, issue)
	}
	return errs
}

// LoadCSV reads a CSV file and unmarshals rows into []T using csv tags or field names.
func LoadCSV[T any](path string) ([]T, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("csv path cannot be empty")
	}

	rowType, err := csvRowType[T]()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read csv file %q failed: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("csv file %q is empty", path)
	}

	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv file %q failed: %w", path, err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("csv file %q is empty", path)
	}

	headers, err := normalizeCSVHeaders(records[0])
	if err != nil {
		return nil, fmt.Errorf("invalid csv header in %q: %w", path, err)
	}
	binding, err := buildCSVBinding(rowType, headers)
	if err != nil {
		return nil, fmt.Errorf("build csv binding for %q failed: %w", path, err)
	}

	rows := make([]T, 0, len(records)-1)
	var issues []CSVCellError

	for recordIndex, record := range records[1:] {
		rowNumber := recordIndex + 2
		if len(record) != len(headers) {
			return nil, fmt.Errorf(
				"csv file %q row %d has %d column(s); expected %d",
				path,
				rowNumber,
				len(record),
				len(headers),
			)
		}

		var item T
		itemValue := reflect.ValueOf(&item).Elem()
		for columnIndex, raw := range record {
			field := binding[columnIndex]
			converted, convertErr := convertCSVValue(raw, field.typ)
			if convertErr != nil {
				issues = append(issues, CSVCellError{
					Row:    rowNumber,
					Column: columnIndex + 1,
					Header: headers[columnIndex],
					Value:  raw,
					Err:    convertErr,
				})
				converted = zeroCSVValue(field.typ)
			}
			itemValue.FieldByIndex(field.index).Set(converted)
		}
		rows = append(rows, item)
	}

	if len(issues) > 0 {
		return rows, &CSVLoadError{
			Path:   path,
			Issues: issues,
		}
	}

	return rows, nil
}

func csvRowType[T any]() (reflect.Type, error) {
	var zero T
	rowType := reflect.TypeOf(zero)
	if rowType == nil {
		return nil, errors.New("csv target type cannot be nil")
	}
	if rowType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("csv target type %s must be a struct", rowType)
	}
	return rowType, nil
}

func normalizeCSVHeaders(rawHeaders []string) ([]string, error) {
	if len(rawHeaders) == 0 {
		return nil, errors.New("header row cannot be empty")
	}

	headers := make([]string, len(rawHeaders))
	seen := make(map[string]struct{}, len(rawHeaders))
	for index, raw := range rawHeaders {
		name := strings.TrimSpace(raw)
		if index == 0 {
			name = strings.TrimPrefix(name, "\ufeff")
		}
		if name == "" {
			return nil, fmt.Errorf("header column %d is empty", index+1)
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("duplicate header %q", name)
		}
		seen[name] = struct{}{}
		headers[index] = name
	}
	return headers, nil
}

func buildCSVBinding(rowType reflect.Type, headers []string) ([]csvFieldBinding, error) {
	fieldMap := make(map[string]csvFieldBinding, len(headers))
	if err := collectCSVFields(rowType, nil, fieldMap); err != nil {
		return nil, err
	}

	binding := make([]csvFieldBinding, len(headers))
	for index, header := range headers {
		field, ok := fieldMap[header]
		if !ok {
			return nil, fmt.Errorf("header %q has no matching exported field in %s", header, rowType)
		}
		if err := validateCSVFieldType(field.typ); err != nil {
			return nil, fmt.Errorf("field for header %q is unsupported: %w", header, err)
		}
		binding[index] = field
	}

	return binding, nil
}

func collectCSVFields(rowType reflect.Type, prefix []int, fieldMap map[string]csvFieldBinding) error {
	for index := 0; index < rowType.NumField(); index++ {
		field := rowType.Field(index)
		if field.PkgPath != "" {
			continue
		}

		tagName, skip := parseCSVFieldTag(field.Tag.Get("csv"))
		if skip {
			continue
		}

		fieldIndex := append(append([]int(nil), prefix...), index)
		if field.Anonymous && tagName == "" && field.Type.Kind() == reflect.Struct {
			if err := collectCSVFields(field.Type, fieldIndex, fieldMap); err != nil {
				return err
			}
			continue
		}

		name := tagName
		if name == "" {
			name = field.Name
		}
		if _, exists := fieldMap[name]; exists {
			return fmt.Errorf("duplicate csv field mapping for %q in %s", name, rowType)
		}
		fieldMap[name] = csvFieldBinding{
			index: fieldIndex,
			typ:   field.Type,
		}
	}

	return nil
}

func parseCSVFieldTag(tag string) (name string, skip bool) {
	if tag == "" {
		return "", false
	}
	name = strings.TrimSpace(strings.Split(tag, ",")[0])
	if name == "-" {
		return "", true
	}
	return name, false
}

func validateCSVFieldType(fieldType reflect.Type) error {
	if supportsCSVScalarType(fieldType) {
		return nil
	}
	if fieldType.Kind() != reflect.Slice {
		return fmt.Errorf("type %s is not supported", fieldType)
	}
	if fieldType.Elem().Kind() == reflect.Slice {
		return fmt.Errorf("nested slice type %s is not supported", fieldType)
	}
	if !supportsCSVScalarType(fieldType.Elem()) {
		return fmt.Errorf("slice element type %s is not supported", fieldType.Elem())
	}
	return nil
}

func supportsCSVScalarType(fieldType reflect.Type) bool {
	if fieldType.Kind() == reflect.Pointer {
		return false
	}
	if fieldType.Implements(textUnmarshalerType) {
		return true
	}
	if reflect.PointerTo(fieldType).Implements(textUnmarshalerType) {
		return true
	}

	switch fieldType.Kind() {
	case reflect.String,
		reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func convertCSVValue(raw string, targetType reflect.Type) (reflect.Value, error) {
	if targetType.Kind() == reflect.Slice {
		return convertCSVSlice(raw, targetType)
	}
	return convertCSVScalar(raw, targetType)
}

func convertCSVScalar(raw string, targetType reflect.Type) (reflect.Value, error) {
	if value, ok, err := tryCSVTextUnmarshal(raw, targetType); ok {
		if err != nil {
			return reflect.Value{}, err
		}
		return value, nil
	}

	value := reflect.New(targetType).Elem()
	trimmed := strings.TrimSpace(raw)

	switch targetType.Kind() {
	case reflect.String:
		value.SetString(raw)
		return value, nil
	case reflect.Bool:
		if trimmed == "" {
			return value, nil
		}
		parsed, err := strconv.ParseBool(trimmed)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("parse bool failed: %w", err)
		}
		value.SetBool(parsed)
		return value, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if trimmed == "" {
			return value, nil
		}
		parsed, err := strconv.ParseInt(trimmed, 10, targetType.Bits())
		if err != nil {
			return reflect.Value{}, fmt.Errorf("parse int failed: %w", err)
		}
		value.SetInt(parsed)
		return value, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if trimmed == "" {
			return value, nil
		}
		parsed, err := strconv.ParseUint(trimmed, 10, targetType.Bits())
		if err != nil {
			return reflect.Value{}, fmt.Errorf("parse uint failed: %w", err)
		}
		value.SetUint(parsed)
		return value, nil
	case reflect.Float32, reflect.Float64:
		if trimmed == "" {
			return value, nil
		}
		parsed, err := strconv.ParseFloat(trimmed, targetType.Bits())
		if err != nil {
			return reflect.Value{}, fmt.Errorf("parse float failed: %w", err)
		}
		value.SetFloat(parsed)
		return value, nil
	default:
		return reflect.Value{}, fmt.Errorf("type %s is not supported", targetType)
	}
}

func tryCSVTextUnmarshal(raw string, targetType reflect.Type) (reflect.Value, bool, error) {
	if targetType.Implements(textUnmarshalerType) {
		value := reflect.New(targetType).Elem()
		unmarshaler, ok := value.Interface().(encoding.TextUnmarshaler)
		if !ok {
			return reflect.Value{}, false, nil
		}
		if err := unmarshaler.UnmarshalText([]byte(raw)); err != nil {
			return reflect.Value{}, true, err
		}
		return value, true, nil
	}

	if reflect.PointerTo(targetType).Implements(textUnmarshalerType) {
		ptr := reflect.New(targetType)
		unmarshaler := ptr.Interface().(encoding.TextUnmarshaler)
		if err := unmarshaler.UnmarshalText([]byte(raw)); err != nil {
			return reflect.Value{}, true, err
		}
		return ptr.Elem(), true, nil
	}

	return reflect.Value{}, false, nil
}

func convertCSVSlice(raw string, targetType reflect.Type) (reflect.Value, error) {
	items, err := parseCSVList(raw)
	if err != nil {
		return zeroCSVValue(targetType), err
	}

	elemType := targetType.Elem()
	value := reflect.MakeSlice(targetType, 0, len(items))
	for index, item := range items {
		converted, convertErr := convertCSVScalar(strings.TrimSpace(item), elemType)
		if convertErr != nil {
			return zeroCSVValue(targetType), fmt.Errorf("parse list element %d failed: %w", index+1, convertErr)
		}
		value = reflect.Append(value, converted)
	}

	return value, nil
}

func parseCSVList(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []string{}, nil
	}
	if trimmed == "[]" {
		return []string{}, nil
	}
	if !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
		return nil, fmt.Errorf("list value %q must be wrapped in []", raw)
	}

	body := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	if body == "" {
		return []string{}, nil
	}

	reader := csv.NewReader(strings.NewReader(body))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	items, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("parse list value %q failed: %w", raw, err)
	}
	return items, nil
}

func zeroCSVValue(targetType reflect.Type) reflect.Value {
	if targetType.Kind() == reflect.Slice {
		return reflect.MakeSlice(targetType, 0, 0)
	}
	return reflect.New(targetType).Elem()
}
