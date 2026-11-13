package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Geofence struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Category    string      `json:"category"`
	Coordinates [][]float64 `json:"coordinates"`
	Active      bool        `json:"active"`
	CreatedAt   time.Time   `json:"created_at"`
}
type Vehicle struct {
	ID            string     `json:"id"`
	VehicleNumber string     `json:"vehicle_number"`
	DriverName    string     `json:"driver_name"`
	VehicleType   string     `json:"vehicle_type"`
	Phone         string     `json:"phone"`
	Latitude      *float64   `json:"latitude,omitempty"`
	Longitude     *float64   `json:"longitude,omitempty"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
}
type Rule struct {
	ID         string    `json:"id"`
	GeofenceID string    `json:"geofence_id"`
	VehicleID  string    `json:"vehicle_id,omitempty"`
	EventType  string    `json:"event_type"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"created_at"`
}
type Violation struct {
	ID            string    `json:"id"`
	VehicleID     string    `json:"vehicle_id"`
	VehicleNumber string    `json:"vehicle_number"`
	DriverName    string    `json:"driver_name"`
	GeofenceID    string    `json:"geofence_id"`
	GeofenceName  string    `json:"geofence_name"`
	Category      string    `json:"category"`
	EventType     string    `json:"event_type"`
	Latitude      float64   `json:"latitude"`
	Longitude     float64   `json:"longitude"`
	Timestamp     time.Time `json:"timestamp"`
}
type timedWriter struct {
	http.ResponseWriter
	started time.Time
	written bool
}

func (w *timedWriter) WriteHeader(code int) {
	if w.written {
		return
	}
	w.written = true
	w.Header().Set("Content-Type", "application/json")
	w.ResponseWriter.WriteHeader(code)
}
func (w *timedWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}
func (w *timedWriter) json(code int, v any) {
	m := map[string]any{}
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			m[k] = val
		}
	default:
		b, _ := json.Marshal(v)
		_ = json.Unmarshal(b, &m)
	}
	m["time_ns"] = strconv.FormatInt(time.Since(w.started).Nanoseconds(), 10)
	b, _ := json.Marshal(m)
	w.WriteHeader(code)
	_, _ = w.Write(b)
}

type Hub struct {
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	broadcast  chan []byte
	clients    map[*websocket.Conn]chan []byte
	shutdown   chan struct{}
	closeOnce  sync.Once
}

func newHub() *Hub {
	h := &Hub{register: make(chan *websocket.Conn), unregister: make(chan *websocket.Conn), broadcast: make(chan []byte, 256), clients: map[*websocket.Conn]chan []byte{}, shutdown: make(chan struct{})}
	go h.run()
	return h
}
func (h *Hub) run() {
	for {
		select {
		case <-h.shutdown:
			for c, q := range h.clients {
				delete(h.clients, c)
				close(q)
				_ = c.Close()
			}
			return
		case c := <-h.register:
			q := make(chan []byte, 32)
			h.clients[c] = q
			go h.write(c, q)
		case c := <-h.unregister:
			if q, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(q)
			}
		case b := <-h.broadcast:
			for c, q := range h.clients {
				select {
				case q <- b:
				default:
					delete(h.clients, c)
					close(q)
					_ = c.Close()
				}
			}
		}
	}
}
func (h *Hub) write(c *websocket.Conn, q chan []byte) {
	defer func() {
		select {
		case h.unregister <- c:
		case <-h.shutdown:
		}
		_ = c.Close()
	}()
	for b := range q {
		_ = c.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if c.WriteMessage(websocket.TextMessage, b) != nil {
			return
		}
	}
}
func (h *Hub) close() { h.closeOnce.Do(func() { close(h.shutdown) }) }
func (h *Hub) remove(c *websocket.Conn) {
	select {
	case h.unregister <- c:
	case <-h.shutdown:
	}
}

type App struct {
	db  *pgxpool.Pool
	hub *Hub
}

func NewApp(db *pgxpool.Pool) *App     { return &App{db: db, hub: newHub()} }
func pointValid(lat, lon float64) bool { return lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180 }
func validCategory(c string) bool {
	return c == "delivery_zone" || c == "restricted_zone" || c == "toll_zone" || c == "customer_area"
}
func validatePolygon(p [][]float64) error {
	if len(p) < 4 {
		return errors.New("coordinates must contain at least four points")
	}
	for _, v := range p {
		if len(v) != 2 || !pointValid(v[0], v[1]) {
			return errors.New("coordinates must be [latitude, longitude] within valid ranges")
		}
	}
	a, b := p[0], p[len(p)-1]
	if a[0] != b[0] || a[1] != b[1] {
		return errors.New("polygon must be closed: first and last coordinates must match")
	}
	return nil
}
func inside(lat, lon float64, p [][]float64) bool {
	if len(p) < 4 {
		return false
	}
	in := false
	for i, j := 0, len(p)-1; i < len(p); j, i = i, i+1 {
		x1, y1 := p[j][1], p[j][0]
		x2, y2 := p[i][1], p[i][0]
		cross := (lon-x1)*(y2-y1) - (lat-y1)*(x2-x1)
		if cross == 0 && lon >= min(x1, x2) && lon <= max(x1, x2) && lat >= min(y1, y2) && lat <= max(y1, y2) {
			return true
		}
		if (y1 > lat) != (y2 > lat) && lon < (x2-x1)*(lat-y1)/(y2-y1)+x1 {
			in = !in
		}
	}
	return in
}
func parse(r *http.Request, v any) error {
	d := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("request must contain one JSON value")
	}
	return nil
}
func (a *App) ServeHTTP(raw http.ResponseWriter, r *http.Request) {
	w := &timedWriter{ResponseWriter: raw, started: time.Now()}
	origin := os.Getenv("CORS_ORIGIN")
	if origin == "" {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,OPTIONS")
	if r.Method == http.MethodOptions {
		w.json(http.StatusOK, map[string]any{"ok": true})
		return
	}
	path := strings.Trim(r.URL.Path, "/")
	if path == "ws/alerts" && r.Method == http.MethodGet {
		a.websocket(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	switch {
	case path == "geofences" && r.Method == http.MethodPost:
		a.createGeofence(ctx, w, r)
	case path == "geofences" && r.Method == http.MethodGet:
		a.listGeofences(ctx, w, r)
	case strings.HasPrefix(path, "geofences/") && r.Method == http.MethodPut:
		a.updateGeofence(ctx, w, r, strings.TrimPrefix(path, "geofences/"))
	case path == "vehicles" && r.Method == http.MethodPost:
		a.createVehicle(ctx, w, r)
	case path == "vehicles" && r.Method == http.MethodGet:
		a.listVehicles(ctx, w)
	case path == "vehicles/location" && r.Method == http.MethodPost:
		a.updateLocation(ctx, w, r)
	case strings.HasPrefix(path, "vehicles/location/") && r.Method == http.MethodGet:
		a.getLocation(ctx, w, strings.TrimPrefix(path, "vehicles/location/"))
	case path == "alerts/configure" && r.Method == http.MethodPost:
		a.configureRule(ctx, w, r)
	case path == "alerts" && r.Method == http.MethodGet:
		a.listRules(ctx, w, r)
	case path == "alerts/unread" && r.Method == http.MethodGet:
		a.unreadAlerts(ctx, w)
	case path == "alerts/read" && r.Method == http.MethodPost:
		a.markAlertsRead(ctx, w, r)
	case path == "violations/history" && r.Method == http.MethodGet:
		a.history(ctx, w, r)
	default:
		w.json(http.StatusNotFound, map[string]any{"error": "route not found"})
	}
}
func (a *App) dbError(w *timedWriter, err error) {
	log.Printf("database request failed: %v", err)
	w.json(http.StatusInternalServerError, map[string]any{"error": "database operation failed"})
}
func (a *App) createGeofence(ctx context.Context, w *timedWriter, r *http.Request) {
	var x Geofence
	if parse(r, &x) != nil || strings.TrimSpace(x.Name) == "" || !validCategory(x.Category) || validatePolygon(x.Coordinates) != nil {
		w.json(400, map[string]any{"error": "invalid geofence; provide name, valid category and a closed polygon of at least four [latitude,longitude] points"})
		return
	}
	coords, _ := json.Marshal(x.Coordinates)
	err := a.db.QueryRow(ctx, `INSERT INTO geofences(name,description,category,coordinates) VALUES($1,$2,$3,$4) RETURNING id::text,name,description,category,coordinates,active,created_at`, strings.TrimSpace(x.Name), x.Description, x.Category, coords).Scan(&x.ID, &x.Name, &x.Description, &x.Category, &coords, &x.Active, &x.CreatedAt)
	if err != nil {
		a.dbError(w, err)
		return
	}
	_ = json.Unmarshal(coords, &x.Coordinates)
	w.json(201, x)
}
func (a *App) listGeofences(ctx context.Context, w *timedWriter, r *http.Request) {
	rows, err := a.db.Query(ctx, `SELECT id::text,name,description,category,coordinates,active,created_at FROM geofences WHERE ($1='' OR category=$1) ORDER BY created_at DESC`, r.URL.Query().Get("category"))
	if err != nil {
		a.dbError(w, err)
		return
	}
	defer rows.Close()
	xs := []Geofence{}
	for rows.Next() {
		var x Geofence
		var b []byte
		if err = rows.Scan(&x.ID, &x.Name, &x.Description, &x.Category, &b, &x.Active, &x.CreatedAt); err != nil {
			a.dbError(w, err)
			return
		}
		_ = json.Unmarshal(b, &x.Coordinates)
		xs = append(xs, x)
	}
	if rows.Err() != nil {
		a.dbError(w, rows.Err())
		return
	}
	w.json(200, map[string]any{"geofences": xs, "count": len(xs)})
}
func (a *App) updateGeofence(ctx context.Context, w *timedWriter, r *http.Request, fenceID string) {
	var x Geofence
	if parse(r, &x) != nil || strings.TrimSpace(x.Name) == "" || !validCategory(x.Category) || validatePolygon(x.Coordinates) != nil {
		w.json(http.StatusBadRequest, map[string]any{"error": "invalid geofence; provide name, valid category and a closed polygon of at least four [latitude,longitude] points"})
		return
	}
	coordinates, _ := json.Marshal(x.Coordinates)
	err := a.db.QueryRow(ctx, `UPDATE geofences SET name=$2,description=$3,category=$4,coordinates=$5 WHERE id=$1 RETURNING id::text,name,description,category,coordinates,active,created_at`, fenceID, strings.TrimSpace(x.Name), x.Description, x.Category, coordinates).Scan(&x.ID, &x.Name, &x.Description, &x.Category, &coordinates, &x.Active, &x.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) || isInvalidUUID(err) {
		w.json(http.StatusNotFound, map[string]any{"error": "geofence not found"})
		return
	}
	if err != nil {
		a.dbError(w, err)
		return
	}
	_ = json.Unmarshal(coordinates, &x.Coordinates)
	w.json(http.StatusOK, x)
}
func (a *App) createVehicle(ctx context.Context, w *timedWriter, r *http.Request) {
	var x Vehicle
	if parse(r, &x) != nil || strings.TrimSpace(x.VehicleNumber) == "" || strings.TrimSpace(x.DriverName) == "" || strings.TrimSpace(x.VehicleType) == "" || strings.TrimSpace(x.Phone) == "" {
		w.json(400, map[string]any{"error": "vehicle_number, driver_name, vehicle_type and phone are required"})
		return
	}
	err := a.db.QueryRow(ctx, `INSERT INTO vehicles(vehicle_number,driver_name,vehicle_type,phone) VALUES($1,$2,$3,$4) RETURNING id::text,vehicle_number,driver_name,vehicle_type,phone`, strings.TrimSpace(x.VehicleNumber), strings.TrimSpace(x.DriverName), strings.TrimSpace(x.VehicleType), strings.TrimSpace(x.Phone)).Scan(&x.ID, &x.VehicleNumber, &x.DriverName, &x.VehicleType, &x.Phone)
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" {
			w.json(409, map[string]any{"error": "vehicle_number already exists"})
			return
		}
		a.dbError(w, err)
		return
	}
	w.json(201, x)
}
func (a *App) listVehicles(ctx context.Context, w *timedWriter) {
	rows, err := a.db.Query(ctx, `SELECT id::text,vehicle_number,driver_name,vehicle_type,phone,latitude,longitude,updated_at FROM vehicles ORDER BY created_at DESC`)
	if err != nil {
		a.dbError(w, err)
		return
	}
	defer rows.Close()
	xs := []Vehicle{}
	for rows.Next() {
		var x Vehicle
		if err = rows.Scan(&x.ID, &x.VehicleNumber, &x.DriverName, &x.VehicleType, &x.Phone, &x.Latitude, &x.Longitude, &x.UpdatedAt); err != nil {
			a.dbError(w, err)
			return
		}
		xs = append(xs, x)
	}
	if rows.Err() != nil {
		a.dbError(w, rows.Err())
		return
	}
	w.json(200, map[string]any{"vehicles": xs, "count": len(xs)})
}
func (a *App) getLocation(ctx context.Context, w *timedWriter, vehicleID string) {
	var v Vehicle
	err := a.db.QueryRow(ctx, `SELECT id::text,vehicle_number,driver_name,vehicle_type,phone,latitude,longitude,updated_at FROM vehicles WHERE id=$1`, vehicleID).Scan(&v.ID, &v.VehicleNumber, &v.DriverName, &v.VehicleType, &v.Phone, &v.Latitude, &v.Longitude, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) || isInvalidUUID(err) {
		w.json(404, map[string]any{"error": "vehicle not found"})
		return
	}
	if err != nil {
		a.dbError(w, err)
		return
	}
	rows, err := a.db.Query(ctx, `SELECT g.id::text,g.name,g.description,g.category,g.coordinates,g.active,g.created_at FROM vehicle_geofence_states s JOIN geofences g ON g.id=s.geofence_id WHERE s.vehicle_id=$1 AND s.is_inside ORDER BY g.name`, vehicleID)
	if err != nil {
		a.dbError(w, err)
		return
	}
	defer rows.Close()
	gs := []Geofence{}
	for rows.Next() {
		var g Geofence
		var b []byte
		if err = rows.Scan(&g.ID, &g.Name, &g.Description, &g.Category, &b, &g.Active, &g.CreatedAt); err != nil {
			a.dbError(w, err)
			return
		}
		_ = json.Unmarshal(b, &g.Coordinates)
		gs = append(gs, g)
	}
	if rows.Err() != nil {
		a.dbError(w, rows.Err())
		return
	}
	w.json(200, map[string]any{"vehicle": v, "current_geofences": gs})
}
func (a *App) configureRule(ctx context.Context, w *timedWriter, r *http.Request) {
	var x Rule
	if parse(r, &x) != nil || x.GeofenceID == "" || (x.EventType != "entry" && x.EventType != "exit" && x.EventType != "both") {
		w.json(400, map[string]any{"error": "geofence_id and event_type (entry, exit or both) are required"})
		return
	}
	var vehicle any
	if x.VehicleID != "" {
		vehicle = x.VehicleID
	}
	err := a.db.QueryRow(ctx, `INSERT INTO alert_configurations(geofence_id,vehicle_id,event_type) VALUES($1,$2,$3) RETURNING id::text,geofence_id::text,COALESCE(vehicle_id::text,''),event_type,active,created_at`, x.GeofenceID, vehicle, x.EventType).Scan(&x.ID, &x.GeofenceID, &x.VehicleID, &x.EventType, &x.Active, &x.CreatedAt)
	if err != nil {
		var pe *pgconn.PgError
		if isInvalidUUID(err) || (errors.As(err, &pe) && pe.Code == "23503") {
			w.json(404, map[string]any{"error": "geofence or vehicle not found"})
			return
		}
		if errors.As(err, &pe) && pe.Code == "23505" {
			w.json(409, map[string]any{"error": "matching alert rule already exists"})
			return
		}
		a.dbError(w, err)
		return
	}
	w.json(201, x)
}
func (a *App) listRules(ctx context.Context, w *timedWriter, r *http.Request) {
	rows, err := a.db.Query(ctx, `SELECT id::text,geofence_id::text,COALESCE(vehicle_id::text,''),event_type,active,created_at FROM alert_configurations WHERE ($1='' OR geofence_id::text=$1) AND ($2='' OR vehicle_id::text=$2 OR vehicle_id IS NULL) ORDER BY created_at DESC`, r.URL.Query().Get("geofence_id"), r.URL.Query().Get("vehicle_id"))
	if err != nil {
		a.dbError(w, err)
		return
	}
	defer rows.Close()
	xs := []Rule{}
	for rows.Next() {
		var x Rule
		if err = rows.Scan(&x.ID, &x.GeofenceID, &x.VehicleID, &x.EventType, &x.Active, &x.CreatedAt); err != nil {
			a.dbError(w, err)
			return
		}
		xs = append(xs, x)
	}
	if rows.Err() != nil {
		a.dbError(w, rows.Err())
		return
	}
	w.json(200, map[string]any{"alerts": xs, "count": len(xs)})
}

func (a *App) unreadAlerts(ctx context.Context, w *timedWriter) {
	rows, err := a.db.Query(ctx, `SELECT id::text FROM violations WHERE read_at IS NULL ORDER BY event_at DESC,id`)
	if err != nil {
		a.dbError(w, err)
		return
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			a.dbError(w, err)
			return
		}
		ids = append(ids, id)
	}
	if rows.Err() != nil {
		a.dbError(w, rows.Err())
		return
	}
	w.json(http.StatusOK, map[string]any{"alert_ids": ids, "count": len(ids)})
}

func (a *App) markAlertsRead(ctx context.Context, w *timedWriter, r *http.Request) {
	var request struct {
		AlertIDs []string `json:"alert_ids"`
	}
	if parse(r, &request) != nil || request.AlertIDs == nil || len(request.AlertIDs) > 1000 {
		w.json(http.StatusBadRequest, map[string]any{"error": "alert_ids must be an array with at most 1000 ids"})
		return
	}
	result, err := a.db.Exec(ctx, `UPDATE violations SET read_at=now() WHERE read_at IS NULL AND id::text=ANY($1::text[])`, request.AlertIDs)
	if err != nil {
		a.dbError(w, err)
		return
	}
	w.json(http.StatusOK, map[string]any{"marked_count": result.RowsAffected()})
}

func (a *App) updateLocation(ctx context.Context, w *timedWriter, r *http.Request) {
	var q struct {
		VehicleID string  `json:"vehicle_id"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}
	if parse(r, &q) != nil || q.VehicleID == "" || !pointValid(q.Latitude, q.Longitude) {
		w.json(400, map[string]any{"error": "valid vehicle_id, latitude and longitude are required"})
		return
	}
	tx, err := a.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		a.dbError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	var v Vehicle
	err = tx.QueryRow(ctx, `SELECT id::text,vehicle_number,driver_name FROM vehicles WHERE id=$1 FOR UPDATE`, q.VehicleID).Scan(&v.ID, &v.VehicleNumber, &v.DriverName)
	if errors.Is(err, pgx.ErrNoRows) || isInvalidUUID(err) {
		w.json(404, map[string]any{"error": "vehicle not found"})
		return
	}
	if err != nil {
		a.dbError(w, err)
		return
	}
	rows, err := tx.Query(ctx, `SELECT id::text,name,description,category,coordinates,active,created_at FROM geofences WHERE active ORDER BY id`)
	if err != nil {
		a.dbError(w, err)
		return
	}
	type fence struct {
		g    Geofence
		poly [][]float64
	}
	fences := []fence{}
	for rows.Next() {
		var f fence
		var b []byte
		if err = rows.Scan(&f.g.ID, &f.g.Name, &f.g.Description, &f.g.Category, &b, &f.g.Active, &f.g.CreatedAt); err != nil {
			rows.Close()
			a.dbError(w, err)
			return
		}
		_ = json.Unmarshal(b, &f.poly)
		fences = append(fences, f)
	}
	rows.Close()
	if rows.Err() != nil {
		a.dbError(w, rows.Err())
		return
	}
	now := time.Now().UTC()
	if _, err = tx.Exec(ctx, `INSERT INTO vehicle_locations(vehicle_id,latitude,longitude,recorded_at) VALUES($1,$2,$3,$4)`, q.VehicleID, q.Latitude, q.Longitude, now); err != nil {
		a.dbError(w, err)
		return
	}
	if _, err = tx.Exec(ctx, `UPDATE vehicles SET latitude=$2,longitude=$3,updated_at=$4 WHERE id=$1`, q.VehicleID, q.Latitude, q.Longitude, now); err != nil {
		a.dbError(w, err)
		return
	}
	events := []Violation{}
	geos := []Geofence{}
	for _, f := range fences {
		insideNow := inside(q.Latitude, q.Longitude, f.poly)
		var was bool
		err = tx.QueryRow(ctx, `SELECT is_inside FROM vehicle_geofence_states WHERE vehicle_id=$1 AND geofence_id=$2`, q.VehicleID, f.g.ID).Scan(&was)
		if errors.Is(err, pgx.ErrNoRows) {
			was = false
		} else if err != nil {
			a.dbError(w, err)
			return
		}
		if _, err = tx.Exec(ctx, `INSERT INTO vehicle_geofence_states(vehicle_id,geofence_id,is_inside,updated_at) VALUES($1,$2,$3,$4) ON CONFLICT(vehicle_id,geofence_id) DO UPDATE SET is_inside=EXCLUDED.is_inside,updated_at=EXCLUDED.updated_at`, q.VehicleID, f.g.ID, insideNow, now); err != nil {
			a.dbError(w, err)
			return
		}
		if insideNow {
			geos = append(geos, f.g)
		}
		eventType := ""
		if !was && insideNow {
			eventType = "entry"
		} else if was && !insideNow {
			eventType = "exit"
		}
		if eventType == "" {
			continue
		}
		var matched bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM alert_configurations WHERE active AND geofence_id=$1 AND (vehicle_id=$2 OR vehicle_id IS NULL) AND (event_type=$3 OR event_type='both'))`, f.g.ID, q.VehicleID, eventType).Scan(&matched)
		if err != nil {
			a.dbError(w, err)
			return
		}
		if !matched {
			continue
		}
		var e Violation
		err = tx.QueryRow(ctx, `INSERT INTO violations(vehicle_id,geofence_id,event_type,latitude,longitude,event_at) VALUES($1,$2,$3,$4,$5,$6) RETURNING id::text,event_at`, q.VehicleID, f.g.ID, eventType, q.Latitude, q.Longitude, now).Scan(&e.ID, &e.Timestamp)
		if err != nil {
			a.dbError(w, err)
			return
		}
		e.VehicleID = q.VehicleID
		e.VehicleNumber = v.VehicleNumber
		e.DriverName = v.DriverName
		e.GeofenceID = f.g.ID
		e.GeofenceName = f.g.Name
		e.Category = f.g.Category
		e.EventType = eventType
		e.Latitude = q.Latitude
		e.Longitude = q.Longitude
		events = append(events, e)
	}
	if err = tx.Commit(ctx); err != nil {
		a.dbError(w, err)
		return
	}
	for _, e := range events {
		b, _ := json.Marshal(map[string]any{"type": "geofence_" + e.EventType, "event": e})
		select {
		case a.hub.broadcast <- b:
		default:
			log.Printf("WebSocket queue full; violation %s is stored", e.ID)
		}
	}
	w.json(200, map[string]any{"location_updated": true, "current_geofences": geos, "alerts": events, "timestamp": now})
}
func (a *App) history(ctx context.Context, w *timedWriter, r *http.Request) {
	limit := 100
	if q := r.URL.Query().Get("limit"); q != "" {
		n, e := strconv.Atoi(q)
		if e != nil || n < 1 {
			w.json(400, map[string]any{"error": "limit must be a positive integer"})
			return
		}
		limit = min(n, 500)
	}
	offset := 0
	if q := r.URL.Query().Get("offset"); q != "" {
		n, e := strconv.Atoi(q)
		if e != nil || n < 0 {
			w.json(400, map[string]any{"error": "offset must be a non-negative integer"})
			return
		}
		offset = n
	}
	var from, to any
	if q := r.URL.Query().Get("from"); q != "" {
		v, e := time.Parse(time.RFC3339, q)
		if e != nil {
			w.json(400, map[string]any{"error": "from must be RFC3339"})
			return
		}
		from = v
	}
	if q := r.URL.Query().Get("to"); q != "" {
		v, e := time.Parse(time.RFC3339, q)
		if e != nil {
			w.json(400, map[string]any{"error": "to must be RFC3339"})
			return
		}
		to = v
	}
	filter := ` WHERE ($1='' OR v.vehicle_id::text=$1) AND ($2='' OR v.geofence_id::text=$2) AND ($3::timestamptz IS NULL OR v.event_at >= $3) AND ($4::timestamptz IS NULL OR v.event_at <= $4)`
	var total int
	if err := a.db.QueryRow(ctx, `SELECT count(*) FROM violations v`+filter, r.URL.Query().Get("vehicle_id"), r.URL.Query().Get("geofence_id"), from, to).Scan(&total); err != nil {
		a.dbError(w, err)
		return
	}
	rows, err := a.db.Query(ctx, `SELECT v.id::text,v.vehicle_id::text,ve.vehicle_number,ve.driver_name,v.geofence_id::text,g.name,g.category,v.event_type,v.latitude,v.longitude,v.event_at FROM violations v JOIN vehicles ve ON ve.id=v.vehicle_id JOIN geofences g ON g.id=v.geofence_id`+filter+` ORDER BY v.event_at DESC LIMIT $5 OFFSET $6`, r.URL.Query().Get("vehicle_id"), r.URL.Query().Get("geofence_id"), from, to, limit, offset)
	if err != nil {
		a.dbError(w, err)
		return
	}
	defer rows.Close()
	xs := []Violation{}
	for rows.Next() {
		var x Violation
		if err = rows.Scan(&x.ID, &x.VehicleID, &x.VehicleNumber, &x.DriverName, &x.GeofenceID, &x.GeofenceName, &x.Category, &x.EventType, &x.Latitude, &x.Longitude, &x.Timestamp); err != nil {
			a.dbError(w, err)
			return
		}
		xs = append(xs, x)
	}
	if rows.Err() != nil {
		a.dbError(w, rows.Err())
		return
	}
	w.json(200, map[string]any{"violations": xs, "total": total, "limit": limit, "offset": offset})
}
func (a *App) websocket(w *timedWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w.ResponseWriter, r, nil)
	if err != nil {
		return
	}
	a.hub.register <- conn
	for {
		if _, _, err = conn.ReadMessage(); err != nil {
			a.hub.remove(conn)
			return
		}
	}
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
	allowed := os.Getenv("CORS_ORIGIN")
	return allowed == "" || allowed == "*" || r.Header.Get("Origin") == allowed
}}

func migrate(ctx context.Context, db *pgxpool.Pool) error {
	if _, err := db.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	files, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		version := path.Base(file)
		tx, err := db.Begin(ctx)
		if err != nil {
			return err
		}
		var applied bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&applied); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if applied {
			_ = tx.Rollback(ctx)
			continue
		}
		contents, err := migrationFiles.ReadFile(file)
		if err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if _, err = tx.Exec(ctx, string(contents)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", version, err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
func isInvalidUUID(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}
func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://fleet:fleet@localhost:5432/fleet?sslmode=disable"
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatal(err)
	}
	cfg.MaxConns = envInt("DB_MAX_CONNS", 20)
	cfg.MinConns = envInt("DB_MIN_CONNS", 2)
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var db *pgxpool.Pool
	for {
		db, err = pgxpool.NewWithConfig(ctx, cfg)
		if err == nil {
			err = db.Ping(ctx)
		}
		if err == nil {
			break
		}
		if db != nil {
			db.Close()
		}
		select {
		case <-ctx.Done():
			log.Fatalf("connect to PostgreSQL: %v", err)
		case <-time.After(2 * time.Second):
		}
	}
	if err = migrate(ctx, db); err != nil {
		log.Fatalf("run migrations: %v", err)
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	app := NewApp(db)
	srv := &http.Server{Addr: addr, Handler: app, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		log.Printf("fleet API listening on %s", addr)
		if e := srv.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
			log.Fatal(e)
		}
	}()
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	stop, done := context.WithTimeout(context.Background(), 15*time.Second)
	defer done()
	if err = srv.Shutdown(stop); err != nil {
		log.Printf("HTTP shutdown: %v", err)
	}
	app.hub.close()
	db.Close()
}
func envInt(key string, def int32) int32 {
	if n, e := strconv.Atoi(os.Getenv(key)); e == nil && n > 0 {
		return int32(n)
	}
	return def
}
