package bencode

import (
	"bytes"
	"fmt"
	"strconv"
)

type BencodeValue struct {
	IntVal    int64
	StringVal []byte
	ListVal   []*BencodeValue
	DictVal   map[string]*BencodeValue
	Type      string
	RawBytes  []byte // the original bytes that produced this value
}

func Decode(data []byte) (*BencodeValue, error) {
	val, _, err := decodeNext(data, 0)
	return val, err
}

func decodeNext(data []byte, pos int) (*BencodeValue, int, error) {
	if pos >= len(data) {
		return nil, pos, fmt.Errorf("Unexpected end of data")
	}

	switch {
	case data[pos] == 'i':
		return decodeInt(data, pos)
	case data[pos] == 'l':
		return decodeList(data, pos)
	case data[pos] == 'd':
		return decodeDict(data, pos)
	case data[pos] >= '0' && data[pos] <= '9':
		return decodeString(data, pos)
	default:
		return nil, pos, fmt.Errorf("unknown type at pos %d: %c", pos, data[pos])

	}
}

func decodeInt(data []byte, pos int) (*BencodeValue, int, error) {
	pos++
	end := bytes.IndexByte(data[pos:], 'e')
	if end == -1 {
		return nil, pos, fmt.Errorf("unterminated integer")
	}
	n, err := strconv.ParseInt(string(data[pos:pos+end]), 10, 64)
	if err != nil {
		return nil, pos, err
	}
	return &BencodeValue{Type: "int", IntVal: n}, pos + end + 1, nil
}

func decodeString(data []byte, pos int) (*BencodeValue, int, error) {
	colon := bytes.IndexByte(data[pos:], ':')
	if colon == -1 {
		return nil, pos, fmt.Errorf("invalid string encoding")
	}
	length, err := strconv.Atoi(string(data[pos : pos+colon]))
	if err != nil {
		return nil, pos, err
	}
	start := pos + colon + 1
	end := start + length
	if end > len(data) {
		return nil, pos, fmt.Errorf("string out of bounds")
	}
	return &BencodeValue{Type: "string", StringVal: data[start:end]}, end, nil
}

func decodeList(data []byte, pos int) (*BencodeValue, int, error) {

	pos++
	var items []*BencodeValue
	for pos < len(data) && data[pos] != 'e' {
		item, newPos, err := decodeNext(data, pos)
		if err != nil {
			return nil, pos, err
		}
		items = append(items, item)
		pos = newPos
	}
	return &BencodeValue{Type: "list", ListVal: items}, pos + 1, nil // +1 to skip 'e'
}

func decodeDict(data []byte, pos int) (*BencodeValue, int, error) {

	pos++
	start := pos - 1
	dict := make(map[string]*BencodeValue)
	for pos < len(data) && data[pos] != 'e' {
		keyVal, newPos, err := decodeString(data, pos)
		if err != nil {
			return nil, pos, fmt.Errorf("dict key error: %w", err)
		}
		pos = newPos
		val, newPos, err := decodeNext(data, pos)
		if err != nil {
			return nil, pos, fmt.Errorf("dict value error: %w", err)
		}
		dict[string(keyVal.StringVal)] = val
		pos = newPos
	}
	return &BencodeValue{Type: "dict", DictVal: dict, RawBytes: data[start : pos+1]}, pos + 1, nil
}
