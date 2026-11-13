package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPolygonValidationAndBoundaries(t *testing.T) {
	poly := [][]float64{{0, 0}, {0, 2}, {2, 2}, {2, 0}, {0, 0}}
	if err := validatePolygon(poly); err != nil {
		t.Fatal(err)
	}
	for _, p := range [][2]float64{{1, 1}, {0, 1}, {2, 2}} {
		if !inside(p[0], p[1], poly) {
			t.Fatalf("expected point %v inside/on polygon", p)
		}
	}
	if inside(3, 1, poly) {
		t.Fatal("outside point considered inside")
	}
	if validatePolygon(poly[:3]) == nil {
		t.Fatal("accepted polygon with fewer than four points")
	}
	open := append([][]float64(nil), poly...)
	open[len(open)-1] = []float64{0, 1}
	if validatePolygon(open) == nil {
		t.Fatal("accepted unclosed polygon")
	}
	if validatePolygon([][]float64{{0, 0}, {0, 2}, {2, 2}, {2, 0}, {0, 0.1}}) == nil {
		t.Fatal("accepted polygon with mismatched end")
	}
	if validatePolygon([][]float64{{0, 0}, {0, 2}, {92, 2}, {0, 0}}) == nil {
		t.Fatal("accepted out of range point")
	}
}

func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; PostgreSQL integration test skipped")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Ping(ctx); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err = migrate(ctx, db); err != nil {
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	return db
}

func request(t *testing.T, c *http.Client, base, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var b bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&b).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, base+path, &b)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var data map[string]any
	if err = json.NewDecoder(res.Body).Decode(&data); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	if _, ok := data["time_ns"].(string); !ok {
		t.Fatalf("%s response missing time_ns: %#v", path, data)
	}
	return res.StatusCode, data
}

func TestPostgresAPITransitionsRealtimeAndPersistence(t *testing.T) {
	db := testDB(t)
	app := NewApp(db)
	t.Cleanup(app.hub.close)
	server := httptest.NewServer(app)
	defer server.Close()
	client := server.Client()
	tag := fmt.Sprintf("test-%d", time.Now().UnixNano())
	poly := [][]float64{{10, 20}, {10, 21}, {11, 21}, {11, 20}, {10, 20}}
	status, g := request(t, client, server.URL, "POST", "/geofences", map[string]any{"name": tag, "description": "integration", "category": "delivery_zone", "coordinates": poly})
	if status != 201 {
		t.Fatalf("create geofence: %d %#v", status, g)
	}
	gid := g["id"].(string)
	t.Cleanup(func() { _, _ = db.Exec(context.Background(), `DELETE FROM geofences WHERE id=$1`, gid) })
	status, glist := request(t, client, server.URL, "GET", "/geofences?category=delivery_zone", nil)
	if status != 200 || glist["count"].(float64) < 1 {
		t.Fatalf("list geofences: %d %#v", status, glist)
	}
	status, v := request(t, client, server.URL, "POST", "/vehicles", map[string]any{"vehicle_number": tag, "driver_name": "Integration Driver", "vehicle_type": "truck", "phone": "555-0100"})
	if status != 201 {
		t.Fatalf("register vehicle: %d %#v", status, v)
	}
	vid := v["id"].(string)
	t.Cleanup(func() { _, _ = db.Exec(context.Background(), `DELETE FROM vehicles WHERE id=$1`, vid) })
	status, vl := request(t, client, server.URL, "GET", "/vehicles", nil)
	if status != 200 || vl["count"].(float64) < 1 {
		t.Fatalf("list vehicles: %d %#v", status, vl)
	}
	status, rule := request(t, client, server.URL, "POST", "/alerts/configure", map[string]any{"geofence_id": gid, "event_type": "both"})
	if status != 201 {
		t.Fatalf("global rule: %d %#v", status, rule)
	}
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/alerts"
	ws1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws1.Close()
	ws2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws2.Close()
	time.Sleep(30 * time.Millisecond)
	status, rules := request(t, client, server.URL, "GET", "/alerts?geofence_id="+gid, nil)
	if status != 200 || rules["count"].(float64) < 1 {
		t.Fatalf("list rules: %d %#v", status, rules)
	}
	status, loc := request(t, client, server.URL, "GET", "/vehicles/location/"+vid, nil)
	if status != 200 {
		t.Fatalf("get location: %d %#v", status, loc)
	}
	status, loc = request(t, client, server.URL, "POST", "/vehicles/location", map[string]any{"vehicle_id": vid, "latitude": 10.5, "longitude": 20.5})
	if status != 200 || loc["location_updated"] != true || len(loc["alerts"].([]any)) != 1 {
		t.Fatalf("entry transition: %d %#v", status, loc)
	}
	for i, conn := range []*websocket.Conn{ws1, ws2} {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, payload, readErr := conn.ReadMessage()
		if readErr != nil {
			t.Fatalf("WebSocket client %d did not receive persisted entry: %v", i, readErr)
		}
		var message map[string]any
		if err = json.Unmarshal(payload, &message); err != nil || message["type"] != "geofence_entry" {
			t.Fatalf("unexpected WebSocket event: %s", payload)
		}
	}
	status, loc = request(t, client, server.URL, "POST", "/vehicles/location", map[string]any{"vehicle_id": vid, "latitude": 10.5, "longitude": 20.5})
	if status != 200 || len(loc["alerts"].([]any)) != 0 {
		t.Fatalf("duplicate event: %d %#v", status, loc)
	}
	status, loc = request(t, client, server.URL, "POST", "/vehicles/location", map[string]any{"vehicle_id": vid, "latitude": 12, "longitude": 22})
	if status != 200 || len(loc["alerts"].([]any)) != 1 {
		t.Fatalf("exit transition: %d %#v", status, loc)
	}
	status, h := request(t, client, server.URL, "GET", "/violations/history?vehicle_id="+vid+"&geofence_id="+gid+"&limit=500", nil)
	if status != 200 || h["total"].(float64) != 2 {
		t.Fatalf("history: %d %#v", status, h)
	}
	// A new API process sees persisted data from the same PostgreSQL database.
	app2 := NewApp(db)
	t.Cleanup(app2.hub.close)
	s2 := httptest.NewServer(app2)
	defer s2.Close()
	status, afterRestart := request(t, client, s2.URL, "GET", "/vehicles/location/"+vid, nil)
	if status != 200 {
		t.Fatalf("after restart read: %d %#v", status, afterRestart)
	}
	status, persistedHistory := request(t, client, s2.URL, "GET", "/violations/history?vehicle_id="+vid+"&limit=500", nil)
	if status != 200 || persistedHistory["total"].(float64) != 2 {
		t.Fatalf("history did not persist after restart: %d %#v", status, persistedHistory)
	}
	// Vehicle-specific rule is accepted and filtered along with global rules.
	status, specific := request(t, client, server.URL, "POST", "/alerts/configure", map[string]any{"geofence_id": gid, "vehicle_id": vid, "event_type": "entry"})
	if status != 201 {
		t.Fatalf("vehicle rule: %d %#v", status, specific)
	}
	status, filtered := request(t, client, server.URL, "GET", "/alerts?vehicle_id="+vid, nil)
	if status != 200 || filtered["count"].(float64) < 2 {
		t.Fatalf("filtered rules: %d %#v", status, filtered)
	}
	status, loc = request(t, client, server.URL, "POST", "/vehicles/location", map[string]any{"vehicle_id": vid, "latitude": 10.5, "longitude": 20.5})
	if status != 200 || len(loc["alerts"].([]any)) != 1 {
		t.Fatalf("global and vehicle-specific rules should emit one matching event: %d %#v", status, loc)
	}
	status, isolatedFence := request(t, client, server.URL, "POST", "/geofences", map[string]any{"name": tag + "-private", "category": "restricted_zone", "coordinates": [][]float64{{30, 40}, {30, 41}, {31, 41}, {31, 40}, {30, 40}}})
	if status != 201 {
		t.Fatalf("create isolated geofence: %d %#v", status, isolatedFence)
	}
	privateID := isolatedFence["id"].(string)
	t.Cleanup(func() { _, _ = db.Exec(context.Background(), `DELETE FROM geofences WHERE id=$1`, privateID) })
	status, specificOnly := request(t, client, server.URL, "POST", "/alerts/configure", map[string]any{"geofence_id": privateID, "vehicle_id": vid, "event_type": "entry"})
	if status != 201 {
		t.Fatalf("vehicle-only rule: %d %#v", status, specificOnly)
	}
	status, loc = request(t, client, server.URL, "POST", "/vehicles/location", map[string]any{"vehicle_id": vid, "latitude": 30.5, "longitude": 40.5})
	privateMatched := false
	if status == 200 {
		for _, item := range loc["alerts"].([]any) {
			event := item.(map[string]any)
			if event["geofence_id"] == privateID && event["event_type"] == "entry" {
				privateMatched = true
			}
		}
	}
	if status != 200 || !privateMatched {
		t.Fatalf("vehicle-specific-only entry should match: %d %#v", status, loc)
	}
	// Missing-entity and malformed requests also carry execution timing.
	status, _ = request(t, client, server.URL, "GET", "/vehicles/location/not-a-uuid", nil)
	if status != 404 {
		t.Fatalf("expected handled error response, got %d", status)
	}
	status, _ = request(t, client, server.URL, "POST", "/vehicles/location", map[string]any{"vehicle_id": vid, "latitude": 91, "longitude": 20})
	if status != 400 {
		t.Fatalf("expected coordinate validation error, got %d", status)
	}
}

func TestWebSocketBroadcastsToMultipleClients(t *testing.T) {
	db := testDB(t)
	app := NewApp(db)
	t.Cleanup(app.hub.close)
	httpServer := httptest.NewServer(app)
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws/alerts"
	first, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	// Wait for both registrations to reach the hub before broadcasting.
	time.Sleep(30 * time.Millisecond)
	payload := []byte(`{"type":"geofence_entry","event":{"id":"integration"}}`)
	app.hub.broadcast <- payload
	for i, c := range []*websocket.Conn{first, second} {
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, got, e := c.ReadMessage()
		if e != nil {
			t.Fatalf("client %d did not receive broadcast: %v", i, e)
		}
		if string(got) != string(payload) {
			t.Fatalf("client %d got %s", i, got)
		}
	}
	// A disconnected subscriber must be removed without blocking broadcasts or
	// affecting ordinary API requests.
	_ = first.Close()
	time.Sleep(30 * time.Millisecond)
	app.hub.broadcast <- payload
	_ = second.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, got, e := second.ReadMessage(); e != nil || string(got) != string(payload) {
		t.Fatalf("remaining WebSocket client failed after disconnect: %s %v", got, e)
	}
	status, result := request(t, httpServer.Client(), httpServer.URL, http.MethodGet, "/vehicles", nil)
	if status != http.StatusOK {
		t.Fatalf("API request failed after WebSocket disconnect: %d %#v", status, result)
	}
}

func TestOverlappingGeofencesAndExitOnlyRules(t *testing.T) {
	db := testDB(t)
	app := NewApp(db)
	t.Cleanup(app.hub.close)
	server := httptest.NewServer(app)
	defer server.Close()
	client := server.Client()
	tag := fmt.Sprintf("overlap-%d", time.Now().UnixNano())
	outer := [][]float64{{10, 20}, {10, 24}, {14, 24}, {14, 20}, {10, 20}}
	inner := [][]float64{{11, 21}, {11, 23}, {13, 23}, {13, 21}, {11, 21}}
	createFence := func(name string, polygon [][]float64) string {
		t.Helper()
		status, result := request(t, client, server.URL, http.MethodPost, "/geofences", map[string]any{"name": name, "category": "delivery_zone", "coordinates": polygon})
		if status != http.StatusCreated {
			t.Fatalf("create fence: %d %#v", status, result)
		}
		id := result["id"].(string)
		t.Cleanup(func() { _, _ = db.Exec(context.Background(), `DELETE FROM geofences WHERE id=$1`, id) })
		return id
	}
	outerID := createFence(tag+"-outer", outer)
	innerID := createFence(tag+"-inner", inner)
	status, vehicle := request(t, client, server.URL, http.MethodPost, "/vehicles", map[string]any{"vehicle_number": tag, "driver_name": "Overlap Driver", "vehicle_type": "van", "phone": "555-0101"})
	if status != http.StatusCreated {
		t.Fatalf("create vehicle: %d %#v", status, vehicle)
	}
	vehicleID := vehicle["id"].(string)
	t.Cleanup(func() { _, _ = db.Exec(context.Background(), `DELETE FROM vehicles WHERE id=$1`, vehicleID) })
	for _, rule := range []map[string]any{{"geofence_id": outerID, "event_type": "both"}, {"geofence_id": innerID, "event_type": "exit"}} {
		status, result := request(t, client, server.URL, http.MethodPost, "/alerts/configure", rule)
		if status != http.StatusCreated {
			t.Fatalf("configure rule: %d %#v", status, result)
		}
	}
	update := func(lat, lon float64) map[string]any {
		t.Helper()
		status, result := request(t, client, server.URL, http.MethodPost, "/vehicles/location", map[string]any{"vehicle_id": vehicleID, "latitude": lat, "longitude": lon})
		if status != http.StatusOK {
			t.Fatalf("update location (%f,%f): %d %#v", lat, lon, status, result)
		}
		return result
	}
	if got := len(update(9, 19)["alerts"].([]any)); got != 0 {
		t.Fatalf("first outside location emitted %d events", got)
	}
	if got := len(update(9, 19)["alerts"].([]any)); got != 0 {
		t.Fatalf("repeated location emitted %d events", got)
	}
	entered := update(12, 22)
	if got := len(entered["current_geofences"].([]any)); got != 2 {
		t.Fatalf("overlapping membership count = %d, want 2", got)
	}
	if events := entered["alerts"].([]any); len(events) != 1 || events[0].(map[string]any)["geofence_id"] != outerID {
		t.Fatalf("entry should match outer both rule only: %#v", events)
	}
	if got := len(update(12, 22)["alerts"].([]any)); got != 0 {
		t.Fatalf("repeated inside location emitted %d events", got)
	}
	exitedInner := update(12, 20.5)
	if events := exitedInner["alerts"].([]any); len(events) != 1 || events[0].(map[string]any)["geofence_id"] != innerID || events[0].(map[string]any)["event_type"] != "exit" {
		t.Fatalf("inner exit-only rule did not match: %#v", events)
	}
	exitedOuter := update(15, 25)
	if events := exitedOuter["alerts"].([]any); len(events) != 1 || events[0].(map[string]any)["geofence_id"] != outerID {
		t.Fatalf("outer exit event missing: %#v", events)
	}
	reentered := update(12, 22)
	if events := reentered["alerts"].([]any); len(events) != 1 || events[0].(map[string]any)["geofence_id"] != outerID || events[0].(map[string]any)["event_type"] != "entry" {
		t.Fatalf("re-entry should match outer rule without duplicate inner entry: %#v", events)
	}
}
