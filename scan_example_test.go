package goqu_test

import (
	"context"
	"fmt"

	"github.com/doug-martin/goqu/v9"
)

func ExampleScanAll() {
	type User struct {
		FirstName string `db:"first_name"`
		LastName  string `db:"last_name"`
	}
	users, err := goqu.ScanAll[User](context.Background(), getDB().From("goqu_user").Order(goqu.C("id").Asc()))
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Printf("%+v\n", users)

	// Output:
	// [{FirstName:Bob LastName:Yukon} {FirstName:Sally LastName:Yukon} {FirstName:Vinita LastName:Yukon} {FirstName:John LastName:Doe}]
}

func ExampleScanOne() {
	type User struct {
		FirstName string `db:"first_name"`
		LastName  string `db:"last_name"`
	}
	for _, name := range []string{"Bob", "Zeb"} {
		ds := getDB().From("goqu_user").Where(goqu.C("first_name").Eq(name))
		user, found, err := goqu.ScanOne[User](context.Background(), ds)
		switch {
		case err != nil:
			fmt.Println(err.Error())
		case !found:
			fmt.Printf("No user found for first_name %s\n", name)
		default:
			fmt.Printf("Found user: %+v\n", user)
		}
	}

	// Output:
	// Found user: {FirstName:Bob LastName:Yukon}
	// No user found for first_name Zeb
}

func ExampleScanVals() {
	ids, err := goqu.ScanVals[int64](context.Background(), getDB().From("goqu_user").Select("id").Order(goqu.C("id").Asc()))
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Printf("UserIds = %+v\n", ids)

	// Output:
	// UserIds = [1 2 3 4]
}

func ExampleScanVal() {
	ds := getDB().From("goqu_user").Select("id").Where(goqu.C("first_name").Eq("Sally"))
	id, found, err := goqu.ScanVal[int64](context.Background(), ds)
	fmt.Println(id, found, err)

	// Output:
	// 2 true <nil>
}
