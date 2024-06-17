package guid_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rancher/rancher/pkg/auth/providers/activedirectory/guid"
)

func TestParse(t *testing.T) {
	tt := []struct {
		name         string
		encoded      []byte
		expectedUUID string
		expectedErr  string
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
			name:        "objectGUID with invalid length",
			encoded:     []byte("\xaf\xf6\x0e\x96\xe3"),
			expectedErr: "invalid length",
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			parsedGUID, err := guid.Parse(tc.encoded)

			if tc.expectedErr != "" {
				assert.ErrorContains(t, err, tc.expectedErr)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tc.expectedUUID, parsedGUID)
			}
		})
	}
}

func TestEncode(t *testing.T) {
	tt := []struct {
		name                string
		uuid                string
		expectedEncodedGUID []byte
		expectedErr         string
	}{
		{
			name:                "valid uuid 1",
			uuid:                "3d0ef6af-965b-44e3-8fea-b23a7d3aa6cb",
			expectedEncodedGUID: []byte("\xaf\xf6\x0e=[\x96\xe3D\x8f\xea\xb2:}:\xa6\xcb"),
		},
		{
			name:                "valid uuid 2",
			uuid:                "75593fbf-57d1-4c55-872d-9372ef0fdd15",
			expectedEncodedGUID: []byte("\xbf?Yu\xd1WUL\x87-\x93r\xef\x0f\xdd\x15"),
		},
		{
			name:                "valid uuid with N char",
			uuid:                "4e4e4e4e-4e4e-4e4e-4e4e-4e4e4e4e4e4e",
			expectedEncodedGUID: []byte("\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e"),
		},
		{
			name:        "invalid uuid",
			uuid:        "75593fbf",
			expectedErr: "invalid format",
		},
		{
			name:        "empty uuid",
			uuid:        "",
			expectedErr: "invalid format",
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := guid.Encode(tc.uuid)

			if tc.expectedErr != "" {
				assert.ErrorContains(t, err, tc.expectedErr)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tc.expectedEncodedGUID, encoded)
			}
		})
	}
}

func TestEscape(t *testing.T) {
	tt := []struct {
		name        string
		objectGUID  []byte
		escapedGUID string
	}{
		{
			name:        "valid objectGUID 1",
			objectGUID:  []byte("\xaf\xf6\x0e=[\x96\xe3D\x8f\xea\xb2:}:\xa6\xcb"),
			escapedGUID: "\\af\\f6\\0e\\3d\\5b\\96\\e3\\44\\8f\\ea\\b2\\3a\\7d\\3a\\a6\\cb",
		},
		{
			name:        "valid objectGUID 2",
			objectGUID:  []byte("\xbf?Yu\xd1WUL\x87-\x93r\xef\x0f\xdd\x15"),
			escapedGUID: "\\bf\\3f\\59\\75\\d1\\57\\55\\4c\\87\\2d\\93\\72\\ef\\0f\\dd\\15",
		},
		{
			name:        "valid objectGUID with N char",
			objectGUID:  []byte("\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e\x4e"),
			escapedGUID: "\\4e\\4e\\4e\\4e\\4e\\4e\\4e\\4e\\4e\\4e\\4e\\4e\\4e\\4e\\4e\\4e",
		},
		{
			name:        "short objectGUID",
			objectGUID:  []byte("a"),
			escapedGUID: "\\61",
		},
		{
			name:        "empty objectGUID",
			objectGUID:  []byte(""),
			escapedGUID: "",
		},
		{
			name:        "nil objectGUID",
			escapedGUID: "",
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			escaped := guid.Escape(tc.objectGUID)
			assert.Equal(t, tc.escapedGUID, escaped)
		})
	}
}
