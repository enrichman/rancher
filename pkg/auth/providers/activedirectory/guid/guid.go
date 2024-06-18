package guid

import (
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	// order defines the bytes arrangement of the original binary objectGUID
	order     = []int{3, 2, 1, 0, 5, 4, 7, 6, 8, 9, 10, 11, 12, 13, 14, 15}
	uuidRegex = regexp.MustCompile("(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")
)

// Parse returns a UUID string in the "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" format
// parsing the encoded binary of an Active Directory objectGUID attribute.
// The encoded byte array should have a length of 16 bytes.
//
// The Microsoft dotnet GUID is a rearrangement of the original binary byte array in this particular order:
//
//	ORDER: [3] [2] [1] [0] - [5] [4] - [7] [6] - [8] [9] - [10] [11] [12] [13] [14] [15]
//
// This can be found in the System/Guid.cs source code (permalink: https://github.com/dotnet/runtime/blob/aa0a7e97764147b0a82412e353003b61b86897d1/src/libraries/System.Private.CoreLib/src/System/Guid.cs#L528-L543)
// and in some blogs and articles.
func Parse(encoded []byte) (string, error) {
	if len(encoded) != 16 {
		return "", errors.New("invalid length")
	}

	ordered := make([]byte, 16)

	for i, pos := range order {
		ordered[pos] = encoded[i]
	}

	return fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		ordered[:4],
		ordered[4:6],
		ordered[6:8],
		ordered[8:10],
		ordered[10:],
	), nil
}

// Encode will return the original Active Directory objectGUID binary format
// encoding the UUID string representation.
// It returns an error if the proved UUID is not valid.
func Encode(uuid string) ([]byte, error) {
	if !uuidRegex.MatchString(uuid) {
		return nil, errors.New("cannot encode UUID to objectGUID: invalid format")
	}

	uuid = strings.ReplaceAll(uuid, "-", "")
	uuidBytes, err := hex.DecodeString(uuid)
	if err != nil {
		return nil, err
	}

	// this should never happen
	if len(uuidBytes) != 16 {
		return nil, errors.New("invalid UUID length")
	}

	ordered := make([]byte, 16)
	for i, b := range uuidBytes {
		ordered[order[i]] = b
	}

	return ordered, nil
}

// Escape returns an escaped string format of the objectGUID that can be safely used
// through the LDAP search. Every byte has to be encoded in an hex string,
// and prefixed with the '\' character. If a byte has a hex encoded string of
// length 1 then it will be prefixed with a '0'.
func Escape(guid []byte) string {
	builder := strings.Builder{}

	for _, b := range guid {
		builder.WriteString(`\`)

		hex := hex.EncodeToString([]byte{b})
		if len(hex) == 1 {
			builder.WriteString("0")
		}
		builder.WriteString(hex)
	}

	return builder.String()
}
