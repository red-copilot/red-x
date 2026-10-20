package mongowire

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

type Element struct {
	Key   string
	Value any
}

type Document []Element

type Binary struct {
	Subtype byte
	Data    []byte
}

type ObjectID [12]byte

const (
	bsonDouble     = 0x01
	bsonString     = 0x02
	bsonDocument   = 0x03
	bsonArray      = 0x04
	bsonBinary     = 0x05
	bsonObjectID   = 0x07
	bsonBool       = 0x08
	bsonDateTime   = 0x09
	bsonNull       = 0x0a
	bsonInt32      = 0x10
	bsonTimestamp  = 0x11
	bsonInt64      = 0x12
	bsonDecimal128 = 0x13
)

func EncodeDocument(doc Document) ([]byte, error) {
	var body []byte
	for _, elem := range doc {
		value, err := encodeValue(elem.Value)
		if err != nil {
			return nil, fmt.Errorf("encode %s: %w", elem.Key, err)
		}
		tag, payload, ok := splitEncodedValue(value)
		if !ok {
			return nil, fmt.Errorf("encode %s: malformed encoded value", elem.Key)
		}
		body = append(body, tag)
		body = append(body, elem.Key...)
		body = append(body, 0)
		body = append(body, payload...)
	}
	body = append(body, 0)
	out := make([]byte, 4+len(body))
	binary.LittleEndian.PutUint32(out[:4], uint32(len(out)))
	copy(out[4:], body)
	return out, nil
}

func splitEncodedValue(raw []byte) (byte, []byte, bool) {
	if len(raw) < 1 {
		return 0, nil, false
	}
	// encodeValue returns {type, payload}; this normalization keeps the caller readable.
	return raw[0], raw[1:], true
}

func encodeValue(v any) ([]byte, error) {
	switch x := v.(type) {
	case float64:
		out := make([]byte, 9)
		out[0] = bsonDouble
		binary.LittleEndian.PutUint64(out[1:], math.Float64bits(x))
		return out, nil
	case string:
		b := []byte(x)
		out := make([]byte, 1+4+len(b)+1)
		out[0] = bsonString
		binary.LittleEndian.PutUint32(out[1:5], uint32(len(b)+1))
		copy(out[5:], b)
		return out, nil
	case Document:
		body, err := EncodeDocument(x)
		if err != nil {
			return nil, err
		}
		return append([]byte{bsonDocument}, body...), nil
	case []any:
		arr := make(Document, 0, len(x))
		for i, item := range x {
			arr = append(arr, Element{Key: fmt.Sprintf("%d", i), Value: item})
		}
		body, err := EncodeDocument(arr)
		if err != nil {
			return nil, err
		}
		return append([]byte{bsonArray}, body...), nil
	case Binary:
		out := make([]byte, 1+4+1+len(x.Data))
		out[0] = bsonBinary
		binary.LittleEndian.PutUint32(out[1:5], uint32(len(x.Data)))
		out[5] = x.Subtype
		copy(out[6:], x.Data)
		return out, nil
	case ObjectID:
		out := make([]byte, 13)
		out[0] = bsonObjectID
		copy(out[1:], x[:])
		return out, nil
	case bool:
		out := []byte{bsonBool, 0}
		if x {
			out[1] = 1
		}
		return out, nil
	case nil:
		return []byte{bsonNull}, nil
	case int:
		if x >= math.MinInt32 && x <= math.MaxInt32 {
			return encodeValue(int32(x))
		}
		return encodeValue(int64(x))
	case int32:
		out := make([]byte, 5)
		out[0] = bsonInt32
		binary.LittleEndian.PutUint32(out[1:], uint32(x))
		return out, nil
	case int64:
		out := make([]byte, 9)
		out[0] = bsonInt64
		binary.LittleEndian.PutUint64(out[1:], uint64(x))
		return out, nil
	case uint64:
		out := make([]byte, 9)
		out[0] = bsonTimestamp
		binary.LittleEndian.PutUint64(out[1:], x)
		return out, nil
	case []string:
		items := make([]any, len(x))
		for i := range x {
			items[i] = x[i]
		}
		return encodeValue(items)
	default:
		return nil, fmt.Errorf("unsupported BSON value %T", v)
	}
}

func DecodeDocument(raw []byte) (map[string]any, error) {
	if len(raw) < 5 {
		return nil, errors.New("document too short")
	}
	length := int(binary.LittleEndian.Uint32(raw[:4]))
	if length < 5 || length > len(raw) {
		return nil, fmt.Errorf("invalid document length %d", length)
	}
	body := raw[4:length]
	if body[len(body)-1] != 0 {
		return nil, errors.New("document missing terminator")
	}
	out := map[string]any{}
	offset := 0
	for offset < len(body)-1 {
		tag := body[offset]
		offset++
		key, next, err := readCString(body, offset)
		if err != nil {
			return nil, err
		}
		offset = next
		value, next, err := decodeValue(tag, body, offset)
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", key, err)
		}
		offset = next
		out[key] = value
	}
	return out, nil
}

func decodeValue(tag byte, body []byte, offset int) (any, int, error) {
	switch tag {
	case bsonDouble:
		if offset+8 > len(body) {
			return nil, offset, errors.New("truncated double")
		}
		return math.Float64frombits(binary.LittleEndian.Uint64(body[offset:])), offset + 8, nil
	case bsonString:
		if offset+4 > len(body) {
			return nil, offset, errors.New("truncated string length")
		}
		n := int(binary.LittleEndian.Uint32(body[offset:]))
		offset += 4
		if n < 1 || offset+n > len(body) || body[offset+n-1] != 0 {
			return nil, offset, errors.New("invalid string")
		}
		return string(body[offset : offset+n-1]), offset + n, nil
	case bsonDocument:
		if offset+4 > len(body) {
			return nil, offset, errors.New("truncated document")
		}
		n := int(binary.LittleEndian.Uint32(body[offset:]))
		if n < 5 || offset+n > len(body) {
			return nil, offset, errors.New("invalid document")
		}
		doc, err := DecodeDocument(body[offset : offset+n])
		return doc, offset + n, err
	case bsonArray:
		doc, next, err := decodeValue(bsonDocument, body, offset)
		if err != nil {
			return nil, offset, err
		}
		m := doc.(map[string]any)
		items := make([]any, 0, len(m))
		for i := 0; i < len(m); i++ {
			items = append(items, m[fmt.Sprintf("%d", i)])
		}
		return items, next, nil
	case bsonBinary:
		if offset+5 > len(body) {
			return nil, offset, errors.New("truncated binary")
		}
		n := int(binary.LittleEndian.Uint32(body[offset:]))
		offset += 4
		subtype := body[offset]
		offset++
		if n < 0 || offset+n > len(body) {
			return nil, offset, errors.New("invalid binary")
		}
		data := append([]byte(nil), body[offset:offset+n]...)
		return Binary{Subtype: subtype, Data: data}, offset + n, nil
	case bsonObjectID:
		if offset+12 > len(body) {
			return nil, offset, errors.New("truncated object id")
		}
		var id ObjectID
		copy(id[:], body[offset:offset+12])
		return id, offset + 12, nil
	case bsonBool:
		if offset+1 > len(body) {
			return nil, offset, errors.New("truncated bool")
		}
		return body[offset] != 0, offset + 1, nil
	case bsonDateTime, bsonInt64, bsonTimestamp:
		if offset+8 > len(body) {
			return nil, offset, errors.New("truncated int64")
		}
		raw := binary.LittleEndian.Uint64(body[offset:])
		return int64(raw), offset + 8, nil
	case bsonNull:
		return nil, offset, nil
	case bsonInt32:
		if offset+4 > len(body) {
			return nil, offset, errors.New("truncated int32")
		}
		return int32(binary.LittleEndian.Uint32(body[offset:])), offset + 4, nil
	case bsonDecimal128:
		if offset+16 > len(body) {
			return nil, offset, errors.New("truncated decimal128")
		}
		return append([]byte(nil), body[offset:offset+16]...), offset + 16, nil
	default:
		return nil, offset, fmt.Errorf("unsupported BSON type 0x%x", tag)
	}
}

func readCString(body []byte, offset int) (string, int, error) {
	for i := offset; i < len(body); i++ {
		if body[i] == 0 {
			return string(body[offset:i]), i + 1, nil
		}
	}
	return "", offset, errors.New("unterminated cstring")
}

func docString(doc map[string]any, key string) string {
	value, _ := doc[key].(string)
	return value
}

func docInt32(doc map[string]any, key string) int32 {
	switch value := doc[key].(type) {
	case int32:
		return value
	case int64:
		return int32(value)
	default:
		return 0
	}
}

func docBool(doc map[string]any, key string) bool {
	value, _ := doc[key].(bool)
	return value
}

func docBinary(doc map[string]any, key string) []byte {
	value, _ := doc[key].(Binary)
	return value.Data
}

func docStringSlice(doc map[string]any, key string) []string {
	raw, _ := doc[key].([]any)
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if value, ok := item.(string); ok {
			out = append(out, value)
		}
	}
	return out
}

func docOK(doc map[string]any) bool {
	switch value := doc["ok"].(type) {
	case float64:
		return value == 1
	case int32:
		return value == 1
	case int64:
		return value == 1
	case bool:
		return value
	default:
		return false
	}
}
