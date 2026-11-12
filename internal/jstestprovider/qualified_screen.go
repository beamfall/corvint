package jstestprovider

import (
	"reflect"

	"github.com/Beamfall/corvint/internal/jstestreceipt"
)

// QualifiedSecretError is jstestreceipt.QualifiedSecretError, kept under its old name.
type QualifiedSecretError = jstestreceipt.QualifiedSecretError

// keepReportersIdentifiers is the identifier set of an EncodeKeepReportersQualification record.
var keepReportersIdentifiers = jstestreceipt.IdentifierMembers{Keys: []string{"configInputs"}, Schema: jstestreceipt.SchemaNames(reflect.TypeOf(KeepReportersQualification{}))}
