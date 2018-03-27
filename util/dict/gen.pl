#!/usr/bin/perl
use strict;
use warnings;
use 5.10.0;

my @types = qw(string int uint);
push @types, "int$_", "uint$_" for (qw(8 16 32 64));

say <<GOCODE;
/* This file was generated using gen.pl and go fmt. */
// dict is a helper function that allow one to easily get concreate values out of a map[string]interface{}
package dict

type M map[string]interface{}

// Dict is to obtain a map[string]interface{} that has already been cast to a M type.
func (m M)Dict(key string)(v M, err error){
    var val interface{}
    var mv map[string]interface{}
    var ok bool
    if val, ok = m[key]; !ok {
        return v, fmt.Errorf("%v value is required.",key)
    }

    if mv, ok = val.(map[string]interface{}); !ok {
        return v, fmt.Errorf("%v value needs to be of type map[string]interface{}.", key)
    }
    return M(mv), nil
}

GOCODE

for my $T ( @types ) {
    my $fnName = ucfirst($T);
    my $fnSliceName = ucfirst($T).'Slice';
    my $T_ptr = '*'.$T;
    say <<GOCODE;
    // $fnName returns the value as a $T type, if it is unable to convert the value it will error. If the default value is not provided, and it can not find the value, it will return the zero value, and an error.
func (m M) $fnName(key string, def $T_ptr)(v $T, err error){
    var val interface{}
    var ok bool
    if val, ok = m[key]; !ok || val == nil{
        if def != nil {
            return \*def, nil
        }
        return v, fmt.Errorf("%v value is required.", key)
    }

    switch placeholder := val.(type) {
        case $T:
            v = placeholder
        case $T_ptr:
            v = \*placeholder
        default:
            return v, fmt.Errorf("%v value needs to be of type ${T}. Value is of type %T", key, val)
    }
    return v, nil
}

func (m M)$fnSliceName(key string)(v []$T, err error){
    var val interface{}
    var ok bool
    if val, ok = m[key]; !ok {
        return v,nil
    }
    if v, ok = val.([]$T); !ok {
        // It's possible that the value is of type []interface and not of our type, so we need to convert each element to the appropriate
        // type first, and then into the this type.
        var iv []interface{}
        if iv, ok = val.([]interface{}); !ok {
            // Could not convert to the generic type, so we don't have the correct thing.
           return v, fmt.Errorf("%v value needs to be of type []${T}. Value is of type %T", key, val)
        }
        for _, value := range iv {
            vt, ok := value.($T);
            if !ok {
               return v, fmt.Errorf("%v value needs to be of type []${T}. Value is of type %T", key, val)
            }
            v = append(v, vt)
        }
    }
    return v, nil
}

GOCODE
}

