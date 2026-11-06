package goqu_test

import (
	"context"
	"fmt"

	"github.com/doug-martin/goqu/v9"
)

func ExampleScanAllStructs() {
	type User struct {
		FirstName string `db:"first_name"`
		LastName  string `db:"last_name"`
	}
	users, err := goqu.ScanAllStructs[User](context.Background(), getDB().From("goqu_user").Order(goqu.C("id").Asc()))
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Printf("%+v\n", users)

	// Output:
	// [{FirstName:Bob LastName:Yukon} {FirstName:Sally LastName:Yukon} {FirstName:Vinita LastName:Yukon} {FirstName:John LastName:Doe}]
}

func ExampleScanOneStruct() {
	type User struct {
		FirstName string `db:"first_name"`
		LastName  string `db:"last_name"`
	}
	for _, name := range []string{"Bob", "Zeb"} {
		ds := getDB().From("goqu_user").Where(goqu.C("first_name").Eq(name))
		user, found, err := goqu.ScanOneStruct[*User](context.Background(), ds)
		switch {
		case err != nil:
			fmt.Println(err.Error())
		case !found:
			fmt.Printf("No user found for first_name %s: %v\n", name, user)
		default:
			fmt.Printf("Found user: %+v\n", *user)
		}
	}

	// Output:
	// Found user: {FirstName:Bob LastName:Yukon}
	// No user found for first_name Zeb: <nil>
}

func ExampleScanAllVals() {
	ids, err := goqu.ScanAllVals[int64](context.Background(), getDB().From("goqu_user").Select("id").Order(goqu.C("id").Asc()))
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Printf("UserIds = %+v\n", ids)

	// Output:
	// UserIds = [1 2 3 4]
}

func ExampleScanOneVal() {
	for _, name := range []string{"Sally", "Zeb"} {
		ds := getDB().From("goqu_user").Select("id").Where(goqu.C("first_name").Eq(name))
		id, found, err := goqu.ScanOneVal[int64](context.Background(), ds)
		switch {
		case err != nil:
			fmt.Println(err.Error())
		case !found:
			fmt.Printf("No id found for user %s\n", name)
		default:
			fmt.Printf("Found userId: %d\n", id)
		}
	}

	// Output:
	// Found userId: 2
	// No id found for user Zeb
}

func ExampleScanAllVals_returning() {
	ds := getDB().Insert("goqu_user").
		Rows(goqu.Record{"first_name": "Jed", "last_name": "Riley", "created": "2020-01-01 00:00:00"}).
		Returning("id")
	ids, err := goqu.ScanAllVals[int64](context.Background(), ds.Executor())
	fmt.Println(ids, err)

	// Output:
	// [5] <nil>
}
