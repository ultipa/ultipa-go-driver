package unit

import (
	"testing"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestRowGet(t *testing.T) {
	// Create typed values
	tv1, _ := gqldb.NewTypedValue("hello")
	tv2, _ := gqldb.NewTypedValue(int64(42))
	tv3, _ := gqldb.NewTypedValue(true)

	row := gqldb.NewRow([]*gqldb.TypedValue{tv1, tv2, tv3})

	// Test Get
	val, err := row.Get(0)
	if err != nil {
		t.Fatalf("Get(0) failed: %v", err)
	}
	if val != "hello" {
		t.Errorf("expected hello, got %v", val)
	}

	val, err = row.Get(1)
	if err != nil {
		t.Fatalf("Get(1) failed: %v", err)
	}
	if val != int64(42) {
		t.Errorf("expected 42, got %v", val)
	}

	val, err = row.Get(2)
	if err != nil {
		t.Fatalf("Get(2) failed: %v", err)
	}
	if val != true {
		t.Errorf("expected true, got %v", val)
	}

	// Test out of range
	_, err = row.Get(3)
	if err == nil {
		t.Error("expected error for out of range index")
	}

	_, err = row.Get(-1)
	if err == nil {
		t.Error("expected error for negative index")
	}
}

func TestRowGetString(t *testing.T) {
	tv1, _ := gqldb.NewTypedValue("test")
	tv2, _ := gqldb.NewTypedValue(int64(123))

	row := gqldb.NewRow([]*gqldb.TypedValue{tv1, tv2})

	str, err := row.GetString(0)
	if err != nil {
		t.Fatalf("GetString(0) failed: %v", err)
	}
	if str != "test" {
		t.Errorf("expected test, got %v", str)
	}

	// Test conversion from int
	str, err = row.GetString(1)
	if err != nil {
		t.Fatalf("GetString(1) failed: %v", err)
	}
	if str != "123" {
		t.Errorf("expected 123, got %v", str)
	}
}

func TestRowGetInt(t *testing.T) {
	tv1, _ := gqldb.NewTypedValue(int64(42))
	tv2, _ := gqldb.NewTypedValue(int32(100))
	tv3, _ := gqldb.NewTypedValue(float64(3.14))

	row := gqldb.NewRow([]*gqldb.TypedValue{tv1, tv2, tv3})

	val, err := row.GetInt(0)
	if err != nil {
		t.Fatalf("GetInt(0) failed: %v", err)
	}
	if val != 42 {
		t.Errorf("expected 42, got %v", val)
	}

	val, err = row.GetInt(1)
	if err != nil {
		t.Fatalf("GetInt(1) failed: %v", err)
	}
	if val != 100 {
		t.Errorf("expected 100, got %v", val)
	}

	// Test conversion from float
	val, err = row.GetInt(2)
	if err != nil {
		t.Fatalf("GetInt(2) failed: %v", err)
	}
	if val != 3 {
		t.Errorf("expected 3, got %v", val)
	}
}

func TestRowGetFloat(t *testing.T) {
	tv1, _ := gqldb.NewTypedValue(float64(3.14))
	tv2, _ := gqldb.NewTypedValue(int64(42))

	row := gqldb.NewRow([]*gqldb.TypedValue{tv1, tv2})

	val, err := row.GetFloat(0)
	if err != nil {
		t.Fatalf("GetFloat(0) failed: %v", err)
	}
	if val != 3.14 {
		t.Errorf("expected 3.14, got %v", val)
	}

	// Test conversion from int
	val, err = row.GetFloat(1)
	if err != nil {
		t.Fatalf("GetFloat(1) failed: %v", err)
	}
	if val != 42.0 {
		t.Errorf("expected 42.0, got %v", val)
	}
}

func TestRowGetBool(t *testing.T) {
	tv1, _ := gqldb.NewTypedValue(true)
	tv2, _ := gqldb.NewTypedValue(false)

	row := gqldb.NewRow([]*gqldb.TypedValue{tv1, tv2})

	val, err := row.GetBool(0)
	if err != nil {
		t.Fatalf("GetBool(0) failed: %v", err)
	}
	if val != true {
		t.Errorf("expected true, got %v", val)
	}

	val, err = row.GetBool(1)
	if err != nil {
		t.Fatalf("GetBool(1) failed: %v", err)
	}
	if val != false {
		t.Errorf("expected false, got %v", val)
	}
}

func TestResponseIsEmpty(t *testing.T) {
	emptyResp := gqldb.NewResponse([]string{"a"}, nil, 0, false, nil, 0)
	if !emptyResp.IsEmpty() {
		t.Error("expected IsEmpty to be true")
	}

	tv, _ := gqldb.NewTypedValue("test")
	nonEmptyResp := gqldb.NewResponse(
		[]string{"a"},
		[]*gqldb.Row{gqldb.NewRow([]*gqldb.TypedValue{tv})},
		1,
		false,
		nil,
		0,
	)
	if nonEmptyResp.IsEmpty() {
		t.Error("expected IsEmpty to be false")
	}
}

func TestResponseFirstLast(t *testing.T) {
	tv1, _ := gqldb.NewTypedValue("first")
	tv2, _ := gqldb.NewTypedValue("middle")
	tv3, _ := gqldb.NewTypedValue("last")

	resp := gqldb.NewResponse(
		[]string{"value"},
		[]*gqldb.Row{
			gqldb.NewRow([]*gqldb.TypedValue{tv1}),
			gqldb.NewRow([]*gqldb.TypedValue{tv2}),
			gqldb.NewRow([]*gqldb.TypedValue{tv3}),
		},
		3,
		false,
		nil,
		0,
	)

	first := resp.First()
	if first == nil {
		t.Fatal("First() returned nil")
	}
	val, _ := first.GetString(0)
	if val != "first" {
		t.Errorf("expected first, got %v", val)
	}

	last := resp.Last()
	if last == nil {
		t.Fatal("Last() returned nil")
	}
	val, _ = last.GetString(0)
	if val != "last" {
		t.Errorf("expected last, got %v", val)
	}

	// Test empty response
	emptyResp := gqldb.NewResponse([]string{"a"}, nil, 0, false, nil, 0)
	if emptyResp.First() != nil {
		t.Error("expected First() to return nil for empty response")
	}
	if emptyResp.Last() != nil {
		t.Error("expected Last() to return nil for empty response")
	}
}

func TestResponseForEach(t *testing.T) {
	tv1, _ := gqldb.NewTypedValue(int64(1))
	tv2, _ := gqldb.NewTypedValue(int64(2))
	tv3, _ := gqldb.NewTypedValue(int64(3))

	resp := gqldb.NewResponse(
		[]string{"num"},
		[]*gqldb.Row{
			gqldb.NewRow([]*gqldb.TypedValue{tv1}),
			gqldb.NewRow([]*gqldb.TypedValue{tv2}),
			gqldb.NewRow([]*gqldb.TypedValue{tv3}),
		},
		3,
		false,
		nil,
		0,
	)

	var sum int64 = 0
	err := resp.ForEach(func(row *gqldb.Row, index int) error {
		val, _ := row.GetInt(0)
		sum += val
		return nil
	})

	if err != nil {
		t.Fatalf("ForEach failed: %v", err)
	}

	if sum != 6 {
		t.Errorf("expected sum 6, got %d", sum)
	}
}

func TestResponseToMaps(t *testing.T) {
	tv1, _ := gqldb.NewTypedValue("Alice")
	tv2, _ := gqldb.NewTypedValue(int64(30))
	tv3, _ := gqldb.NewTypedValue("Bob")
	tv4, _ := gqldb.NewTypedValue(int64(25))

	resp := gqldb.NewResponse(
		[]string{"name", "age"},
		[]*gqldb.Row{
			gqldb.NewRow([]*gqldb.TypedValue{tv1, tv2}),
			gqldb.NewRow([]*gqldb.TypedValue{tv3, tv4}),
		},
		2,
		false,
		nil,
		0,
	)

	maps, err := resp.ToMaps()
	if err != nil {
		t.Fatalf("ToMaps failed: %v", err)
	}

	if len(maps) != 2 {
		t.Fatalf("expected 2 maps, got %d", len(maps))
	}

	if maps[0]["name"] != "Alice" {
		t.Errorf("expected Alice, got %v", maps[0]["name"])
	}

	if maps[0]["age"] != int64(30) {
		t.Errorf("expected 30, got %v", maps[0]["age"])
	}

	if maps[1]["name"] != "Bob" {
		t.Errorf("expected Bob, got %v", maps[1]["name"])
	}
}

func TestResponseSingleValue(t *testing.T) {
	tv, _ := gqldb.NewTypedValue(int64(42))
	resp := gqldb.NewResponse(
		[]string{"count"},
		[]*gqldb.Row{gqldb.NewRow([]*gqldb.TypedValue{tv})},
		1,
		false,
		nil,
		0,
	)

	val, err := resp.SingleValue()
	if err != nil {
		t.Fatalf("SingleValue failed: %v", err)
	}

	if val != int64(42) {
		t.Errorf("expected 42, got %v", val)
	}

	// Test SingleInt
	intVal, err := resp.SingleInt()
	if err != nil {
		t.Fatalf("SingleInt failed: %v", err)
	}
	if intVal != 42 {
		t.Errorf("expected 42, got %d", intVal)
	}
}

func TestResponseGetByName(t *testing.T) {
	tv1, _ := gqldb.NewTypedValue("Alice")
	tv2, _ := gqldb.NewTypedValue(int64(30))

	resp := gqldb.NewResponse(
		[]string{"name", "age"},
		[]*gqldb.Row{gqldb.NewRow([]*gqldb.TypedValue{tv1, tv2})},
		1,
		false,
		nil,
		0,
	)

	row := resp.First()

	val, err := resp.GetByName(row, "name")
	if err != nil {
		t.Fatalf("GetByName failed: %v", err)
	}
	if val != "Alice" {
		t.Errorf("expected Alice, got %v", val)
	}

	val, err = resp.GetByName(row, "age")
	if err != nil {
		t.Fatalf("GetByName failed: %v", err)
	}
	if val != int64(30) {
		t.Errorf("expected 30, got %v", val)
	}

	// Test non-existent column
	_, err = resp.GetByName(row, "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent column")
	}
}
