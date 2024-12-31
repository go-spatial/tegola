// Copyright (C) 2018, Michael P. Gerlek (Flaxen Consulting)
//
// Portions of this code were derived from the PROJ.4 software
// In keeping with the terms of the PROJ.4 project, this software
// is provided under the MIT-style license in `LICENSE.md` and may
// additionally be subject to the copyrights of the PROJ.4 authors.

package core

import (
	"fmt"

	"github.com/go-spatial/proj/merror"
)

// OperationDescriptionTable is the global list of all the known operations
//
// The algorithms in the operations package call the Register functions
// which populate this map.
var OperationDescriptionTable = map[string]*OperationDescription{}

// ConvertLPToXYCreatorFuncType is the type of the function which creates an operation-specific object
//
// This kind of function, when executed, creates an operation-specific object
// which implements IConvertLPToXY.
type ConvertLPToXYCreatorFuncType func(*System, *OperationDescription) (IConvertLPToXY, error)

// OperationDescription stores the information about a particular kind of
// operation. It is populated from each op in the "operations" package
// into the global table.
type OperationDescription struct {
	ID            string
	Description   string
	Description2  string
	OperationType OperationType
	InputType     CoordType
	OutputType    CoordType
	creatorFunc   interface{} // for now, this will always be a ConvertLPToXYCreatorFuncType
}

// RegisterConvertLPToXY adds an OperationDescription entry to the OperationDescriptionTable
//
// Each file in the operations package has an init() routine which calls this function.
// Each operation supplies its own "creatorFunc", of its own particular type.
func RegisterConvertLPToXY(
	id string,
	description string,
	description2 string,
	creatorFunc ConvertLPToXYCreatorFuncType,
) {
	pi := &OperationDescription{
		ID:            id,
		Description:   description,
		Description2:  description2,
		OperationType: OperationTypeConversion,
		InputType:     CoordTypeLP,
		OutputType:    CoordTypeXY,
		creatorFunc:   creatorFunc,
	}

	_, ok := OperationDescriptionTable[id]
	if ok {
		panic(fmt.Sprintf("duplicate operation description id '%s' : %s", id, description))
	}
	OperationDescriptionTable[id] = pi
}

// CreateOperation returns a new object of the specific operation type, e.g. an operations.EtMerc
func (desc *OperationDescription) CreateOperation(sys *System) (IOperation, error) {

	if desc.IsConvertLPToXY() {
		return NewConvertLPToXY(sys, desc)
	}

	return nil, merror.New(merror.NotYetSupported)
}

// IsConvertLPToXY returns true iff the operation can be cast to an IConvertLPToXY
func (desc *OperationDescription) IsConvertLPToXY() bool {
	return desc.OperationType == OperationTypeConversion &&
		desc.InputType == CoordTypeLP &&
		desc.OutputType == CoordTypeXY
}
