// Copyright (C) 2018, Michael P. Gerlek (Flaxen Consulting)
//
// Portions of this code were derived from the PROJ.4 software
// In keeping with the terms of the PROJ.4 project, this software
// is provided under the MIT-style license in `LICENSE.md` and may
// additionally be subject to the copyrights of the PROJ.4 authors.

package core

// OperationType is the enum for the different kinds of conversions and transforms
//
// This may turn out not to be as useful as it originally seemed.
type OperationType int

// The operation type
const (
	OperationTypeInvalid OperationType = iota
	OperationTypeConversion
	OperationTypeTransformation
)

// IOperation is what all the operations need to support
type IOperation interface {
	GetSystem() *System
	GetDescription() *OperationDescription
}

// Operation is the base class for all operations
type Operation struct {
	System      *System
	Description *OperationDescription
}

// GetSystem returns the System object the operation is associated with
func (op *Operation) GetSystem() *System {
	return op.System
}

// GetDescription returns the OperationDescription of the operation
func (op *Operation) GetDescription() *OperationDescription {
	return op.Description
}
