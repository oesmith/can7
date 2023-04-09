package mbe

import (
	"bytes"
	"encoding/hex"
	"fmt"
)

const (
	ID_EASIMAP uint32 = 0xcbe1101
	ID_ECU     uint32 = 0xcbe0111
)

var VersionReq = []byte{0x4, 0x0, 0xd}
var VersionResPrefix = []byte{0xe4, 0x0, 0xd}

var DataReqPrefix = []byte{0x01, 0x0, 0x0, 0x0, 0x0}
var DataResPrefix = []byte{0x81}

func ParseVersionResponse(d []byte) (ver string, err error) {
	if !bytes.HasPrefix(d, VersionResPrefix) {
		return "", fmt.Errorf("Bad version response: %s", hex.EncodeToString(d))
	}
	return string(d[len(VersionResPrefix):]), nil
}

func CreateDataRequest(page byte, offsets []byte) []byte {
	d := make([]byte, len(DataReqPrefix) + 1 + len(offsets))
	copy(d, DataReqPrefix)
	d[len(DataReqPrefix)] = page
	copy(d[len(DataReqPrefix)+1:], offsets)
	return d
}

func ParseDataResponse(d []byte) ([]byte, error) {
	if !bytes.HasPrefix(d, DataResPrefix) {
		return nil, fmt.Errorf("Bad data response: %s", hex.EncodeToString(d))
	}
	return d[len(DataResPrefix):], nil
}
