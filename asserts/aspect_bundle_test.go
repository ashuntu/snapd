// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2024 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package asserts_test

import (
	"strings"
	"time"

	. "gopkg.in/check.v1"

	"github.com/snapcore/snapd/asserts"
)

type aspectBundleSuite struct {
	ts     time.Time
	tsLine string
}

var _ = Suite(&aspectBundleSuite{})

func (s *aspectBundleSuite) SetUpSuite(c *C) {
	s.ts = time.Now().Truncate(time.Second).UTC()
	s.tsLine = "timestamp: " + s.ts.Format(time.RFC3339) + "\n"
}

const (
	aspectBundleExample = `type: aspect-bundle
authority-id: brand-id1
account-id: brand-id1
name: my-network
summary: aspect-bundle description
aspects:
  wifi-setup:
    rules:
      -
        request: ssids
        storage: wifi.ssids
      -
        request: ssid
        storage: wifi.ssid
        access: read-write
      -
        request: password
        storage: wifi.psk
        access: write
      -
        request: status
        storage: wifi.status
        access: read
      -
        request: private.{key}
        storage: wifi.{key}
storage:
    {
      "schema": {
        "wifi": {
          "type": "map",
          "values": "any"
        }
      }
    }
` + "TSLINE" +
		"body-length: 0\n" +
		"sign-key-sha3-384: Jv8_JiHiIzJVcO9M55pPdqSDWUvuhfDIBJUS-3VW7F_idjix7Ffn5qMxB21ZQuij" +
		"\n\n" +
		"AXNpZw=="
)

func (s *aspectBundleSuite) TestDecodeOK(c *C) {
	encoded := strings.Replace(aspectBundleExample, "TSLINE", s.tsLine, 1)

	a, err := asserts.Decode([]byte(encoded))
	c.Assert(err, IsNil)
	c.Check(a, NotNil)
	c.Check(a.Type(), Equals, asserts.AspectBundleType)
	ab := a.(*asserts.AspectBundle)
	c.Check(ab.AuthorityID(), Equals, "brand-id1")
	c.Check(ab.AccountID(), Equals, "brand-id1")
	c.Check(ab.Name(), Equals, "my-network")
	bundle := ab.Bundle()
	c.Assert(bundle, NotNil)
	c.Check(bundle.Aspect("wifi-setup"), NotNil)
}

func (s *aspectBundleSuite) TestDecodeInvalid(c *C) {
	const validationSetErrPrefix = "assertion aspect-bundle: "

	encoded := strings.Replace(aspectBundleExample, "TSLINE", s.tsLine, 1)

	aspectsStanza := encoded[strings.Index(encoded, "aspects:") : strings.Index(encoded, "\nstorage:")+1]
	storageStanza := encoded[strings.Index(encoded, "\nstorage:")+1 : strings.Index(encoded, "timestamp:")]

	invalidTests := []struct{ original, invalid, expectedErr string }{
		{"account-id: brand-id1\n", "", `"account-id" header is mandatory`},
		{"account-id: brand-id1\n", "account-id: \n", `"account-id" header should not be empty`},
		{"account-id: brand-id1\n", "account-id: random\n", `authority-id and account-id must match, aspect-bundle assertions are expected to be signed by the issuer account: "brand-id1" != "random"`},
		{"name: my-network\n", "", `"name" header is mandatory`},
		{"name: my-network\n", "name: \n", `"name" header should not be empty`},
		{"name: my-network\n", "name: my/network\n", `"name" primary key header cannot contain '/'`},
		{"name: my-network\n", "name: my+network\n", `"name" header contains invalid characters: "my\+network"`},
		{s.tsLine, "", `"timestamp" header is mandatory`},
		{aspectsStanza, "aspects: foo\n", `"aspects" header must be a map`},
		{aspectsStanza, "", `"aspects" stanza is mandatory`},
		{"read-write", "update", `cannot define aspect "wifi-setup": cannot create aspect rule:.*`},
		{storageStanza, "", `"storage" stanza is mandatory`},
		{storageStanza, "storage:\n  - foo\n", `invalid "storage" schema stanza, expected schema text`},
		{storageStanza, "storage:\n    {}\n", `invalid "storage" schema stanza: cannot parse top level schema: must have a "schema" constraint`},
	}

	for _, test := range invalidTests {
		invalid := strings.Replace(encoded, test.original, test.invalid, 1)
		_, err := asserts.Decode([]byte(invalid))
		c.Check(err, ErrorMatches, validationSetErrPrefix+test.expectedErr)
	}
}
