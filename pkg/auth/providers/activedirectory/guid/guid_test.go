package guid_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rancher/rancher/pkg/auth/providers/activedirectory/guid"
)

func TestGuid(t *testing.T) {
	tt := []struct {
		name               string
		encoded            []byte
		expectedUUID       string
		expectedParseErr   string
		expectedEncodedErr string
	}{
		{
			name:         "valid objectGUID 1",
			encoded:      []byte("\xaf\xf6\x0e=[\x96\xe3D\x8f\xea\xb2:}:\xa6\xcb"),
			expectedUUID: "3d0ef6af-965b-44e3-8fea-b23a7d3aa6cb",
		},
		{
			name:         "valid objectGUID 2",
			encoded:      []byte("\xbf?Yu\xd1WUL\x87-\x93r\xef\x0f\xdd\x15"),
			expectedUUID: "75593fbf-57d1-4c55-872d-9372ef0fdd15",
		},
		{
			name:         "valid objectGUID with N char",
			encoded:      []byte("\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e"),
			expectedUUID: "4e4e4e4e-4e4e-4e4e-4e4e-4e4e4e4e4e4e",
		},
		{
			name:               "objectGUID with invalid length",
			encoded:            []byte("\xaf\xf6\x0e\x96\xe3"),
			expectedParseErr:   "invalid length",
			expectedEncodedErr: "invalid length",
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			parsedGUID, err := guid.Parse(tc.encoded)

			if tc.expectedParseErr != "" {
				assert.ErrorContains(t, err, tc.expectedParseErr)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tc.expectedUUID, parsedGUID)
			}

			// test that encoding back works returning the same bytes
			encoded, err := guid.Encode(parsedGUID)

			if tc.expectedEncodedErr != "" {
				assert.Error(t, err, tc.expectedEncodedErr)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tc.encoded, encoded)
			}
		})
	}
}
