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
    if val, ok = m[key]; !ok {
        if def != nil {
            return \*def, nil
        }
        return v, fmt.Errorf("%v value is required.",key)
    }
    if v, ok = val.($T); !ok {
        return \*def, fmt.Errorf("%v value needs to be of type ${T}. Value is of type %T", key, val)
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
        return v, fmt.Errorf("%v value needs to be of type []${T}. Value is of type %T", key, val)
    }
    return v, nil
}

GOCODE
}

