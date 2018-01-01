// Code generated by paladin. DO NOT EDIT.

package dbc

import (
	"encoding/json"
	"frm/plog"
	"os"
)

type Location struct {
	Id           int
	Name         string
	X            int
	Y            int
	Rate1        int
	Rate2        int
	SpellId1     int
	SpellDamage1 int
}

var tblLocation map[int64]*Location

func GetLocation(id int64) *Location {
	return tblLocation[id]
}
func GetAllLocation() map[int64]*Location {
	return tblLocation
}

func init() {
	file, err := os.Open("bin/dbc/location.json")
	if err != nil {
		plog.Error(err)
		return
	}
	decoder := json.NewDecoder(file)
	if err = decoder.Decode(&tblLocation); err != nil {
		plog.Error(err)
	}
}
