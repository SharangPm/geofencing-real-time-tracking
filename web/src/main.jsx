import React, { lazy, Suspense, useEffect, useState, useCallback } from "react";
import { createRoot } from "react-dom/client";
import {
  BrowserRouter,
  NavLink,
  Route,
  Routes,
  useLocation,
  useNavigate,
} from "react-router-dom";
import {
  Container,
  Row,
  Col,
  Card,
  Button,
  Form,
  Table,
  Badge,
  Modal,
  Toast,
  ToastContainer,
  Spinner,
  InputGroup,
} from "react-bootstrap";
import {
  LayoutDashboard,
  Map,
  MapPinned,
  Truck,
  SlidersHorizontal,
  Bell,
  History,
  Search,
  Menu,
  Plus,
  Pencil,
  Radio,
  RefreshCw,
  MapPin,
  Activity,
  AlertTriangle,
  ChevronRight,
} from "lucide-react";
import axios from "axios";
import "bootstrap/dist/css/bootstrap.min.css";
import "./style.css";
const MapView = lazy(() =>
    import("./MapComponents.jsx").then((m) => ({ default: m.MapView })),
  ),
  GeofenceMap = lazy(() =>
    import("./MapComponents.jsx").then((m) => ({ default: m.GeofenceMap })),
  ),
  ZoneChart = lazy(() => import("./ZoneChart.jsx"));
const apiBase =
  import.meta.env.VITE_API_URL ||
  (import.meta.env.PROD ? "/api" : "http://localhost:8080");
const api = axios.create({ baseURL: apiBase });
const menus = [
  ["Dashboard", "/", LayoutDashboard],
  ["Live tracking", "/tracking", Map],
  ["Geofences", "/geofences", MapPinned],
  ["Vehicles", "/vehicles", Truck],
  ["Alert rules", "/rules", SlidersHorizontal],
  ["Live alerts", "/alerts", Bell],
  ["History", "/history", History],
];
const colors = {
  restricted_zone: "#e35462",
  delivery_zone: "#3974d5",
  toll_zone: "#ae78dd",
  customer_area: "#2ca985",
};
const fmt = (x) => (x ? new Date(x).toLocaleString() : "—");
function App() {
  const currentPath = useLocation().pathname;
  const navigate = useNavigate();
  const [collapsed, setCollapsed] = useState(false),
    [online, setOnline] = useState(false),
    [alerts, setAlerts] = useState([]),
    [toast, setToast] = useState(""),
    [refresh, setRefresh] = useState(0);
  useEffect(() => {
    let ws;
    let retry;
    const connect = () => {
      ws = new WebSocket(
        (location.protocol === "https:" ? "wss://" : "ws://") +
          (apiBase.startsWith("http") ? new URL(apiBase).host : location.host) +
          "/ws/alerts",
      );
      ws.onopen = () => setOnline(true);
      ws.onclose = () => {
        setOnline(false);
        retry = setTimeout(connect, 2500);
      };
      ws.onerror = () => ws.close();
      ws.onmessage = (e) => {
        try {
          let m = JSON.parse(e.data),
            a = m.event || m;
          setAlerts((old) => [a, ...old].slice(0, 50));
          setRefresh((x) => x + 1);
          setToast(
            `${a.event_type || m.type} · ${a.vehicle_number} ${a.geofence_name}`,
          );
        } catch {}
      };
    };
    connect();
    return () => {
      clearTimeout(retry);
      ws?.close();
    };
  }, []);
  const notify = (s) => setToast(s);
  return (
    <div className="shell">
      <aside className={"sidebar " + (collapsed ? "slim" : "")}>
        <div className="brand">
          <div className="brand-icon">
            <Radio size={21} />
          </div>
          {!collapsed && (
            <div>
              <b>
                Fleet<span>Ops</span>
              </b>
              <small>CONTROL CENTER</small>
            </div>
          )}
        </div>
        <div className="nav-label">WORKSPACE</div>
        {menus.map(([n, to, I]) => (
          <NavLink
            key={to}
            to={to}
            end={to === "/"}
            title={n}
            className={({ isActive }) =>
              "navitem " + (isActive ? "active" : "")
            }
          >
            <I size={18} />
            {!collapsed && <span>{n}</span>}
            {n === "Live alerts" && alerts.length > 0 && !collapsed && (
              <i className="nav-count">{alerts.length}</i>
            )}
          </NavLink>
        ))}
        <div className="side-bottom">
          <div className="live-dot" />
          {!collapsed && <span>System operational</span>}
        </div>
      </aside>
      <main className="main">
        <header className="topbar">
          <Button
            variant="light"
            className="icon-btn"
            onClick={() => setCollapsed(!collapsed)}
          >
            <Menu size={19} />
          </Button>
          <div className="crumb">
            FLEET MANAGEMENT <ChevronRight size={13} />{" "}
            <b>{menus.find((x) => x[1] === currentPath)?.[0] || "Dashboard"}</b>
          </div>
          <div className="top-actions">
            <span className={"ws-status " + (online ? "good" : "")}>
              <i />
              {online ? "Live connected" : "Connecting"}
            </span>
            <Button
              variant="light"
              className="icon-btn bell"
              onClick={() => navigate("/alerts")}
            >
              <Bell size={18} />
              {alerts.length > 0 && <em />}
            </Button>
            <div className="avatar">FO</div>
          </div>
        </header>
        <div className="page-wrap">
          <Routes>
            <Route
              path="/"
              element={
                <Dashboard refresh={refresh} alerts={alerts} go={notify} />
              }
            />
            <Route
              path="/tracking"
              element={<Tracking refresh={refresh} notify={notify} />}
            />
            <Route
              path="/geofences"
              element={
                <Geofences
                  notify={notify}
                  refreshAll={() => setRefresh((x) => x + 1)}
                />
              }
            />
            <Route
              path="/vehicles"
              element={
                <Vehicles
                  notify={notify}
                  refreshAll={() => setRefresh((x) => x + 1)}
                />
              }
            />
            <Route
              path="/rules"
              element={<Rules notify={notify} refresh={refresh} />}
            />
            <Route
              path="/alerts"
              element={<AlertPage alerts={alerts} refresh={refresh} />}
            />
            <Route
              path="/history"
              element={<HistoryPage refresh={refresh} />}
            />
          </Routes>
        </div>
      </main>
      <ToastContainer position="bottom-end" className="p-3">
        <Toast
          show={!!toast}
          onClose={() => setToast("")}
          delay={4000}
          autohide
          bg="light"
        >
          <Toast.Header>
            <strong className="me-auto">FleetOps notification</strong>
          </Toast.Header>
          <Toast.Body>{toast}</Toast.Body>
        </Toast>
      </ToastContainer>
    </div>
  );
}
function useData(refresh) {
  const [data, setData] = useState({
      vehicles: [],
      geofences: [],
      alerts: [],
      violations: [],
    }),
    [loading, setLoading] = useState(true),
    [error, setError] = useState("");
  const load = () => {
    setLoading(true);
    setError("");
    Promise.all([
      api.get("/vehicles"),
      api.get("/geofences"),
      api.get("/alerts"),
      api.get("/violations/history?limit=500"),
    ])
      .then(([v, g, a, h]) =>
        setData({
          vehicles: v.data.vehicles || [],
          geofences: g.data.geofences || [],
          alerts: a.data.alerts || [],
          violations: h.data.violations || [],
        }),
      )
      .catch((e) => setError(e.response?.data?.error || e.message))
      .finally(() => setLoading(false));
  };
  useEffect(() => {
    load();
  }, [refresh]);
  return { ...data, loading, error, load };
}
function Header({ title, sub, action }) {
  return (
    <div className="page-head">
      <div>
        <div className="eyebrow">
          FLEET OVERVIEW <span>•</span>{" "}
          {new Date().toLocaleDateString(undefined, {
            weekday: "long",
            month: "long",
            day: "numeric",
          })}
        </div>
        <h1>{title}</h1>
        <p>{sub}</p>
      </div>
      {action}
    </div>
  );
}
function Dashboard({ refresh, alerts }) {
  const d = useData(refresh),
    [todayViolations, setTodayViolations] = useState(null),
    [todayError, setTodayError] = useState("");
  useEffect(() => {
    let live = true;
    const start = new Date();
    start.setHours(0, 0, 0, 0);
    const end = new Date();
    end.setHours(23, 59, 59, 999);
    api
      .get("/violations/history", {
        params: { from: start.toISOString(), to: end.toISOString(), limit: 1 },
      })
      .then((r) => {
        if (live) {
          setTodayViolations(r.data.total);
          setTodayError("");
        }
      })
      .catch((e) => {
        if (live) setTodayError(e.response?.data?.error || e.message);
      });
    return () => {
      live = false;
    };
  }, [refresh]);
  const stats = [
    ["Total vehicles", d.vehicles.length, Truck, "blue", "Registered fleet"],
    [
      "Active geofences",
      d.geofences.filter((x) => x.active).length,
      MapPinned,
      "green",
      "Monitoring zones",
    ],
    [
      "Today's violations",
      todayViolations,
      AlertTriangle,
      "orange",
      "Entry & exit events",
    ],
    [
      "Active alert rules",
      d.alerts.filter((x) => x.active).length,
      Bell,
      "purple",
      "Rules enabled",
    ],
  ];
  const counts = [
    "delivery_zone",
    "restricted_zone",
    "toll_zone",
    "customer_area",
  ].map((k) => ({
    name: k.replace("_zone", "").replace("_", " "),
    count: d.geofences.filter((g) => g.category === k).length,
  }));
  return (
    <>
      <Header
        title="Good day, Operator"
        sub="Monitor your fleet, geofences and recent activity at a glance."
      />
      <Row className="g-3 mb-4">
        {stats.map(([n, v, I, c, s]) => (
          <Col xl={3} sm={6} key={n}>
            <Card className="stat-card">
              <div className={"stat-icon " + c}>
                <I size={19} />
              </div>
              <div className="stat-label">{n}</div>
              <div className="stat-value">
                {d.loading || v === null ? <Spinner size="sm" /> : v}
              </div>
              <div className="stat-foot">{s}</div>
            </Card>
          </Col>
        ))}
      </Row>
      {(d.error || todayError) && <ErrorBox error={d.error || todayError} />}
      <Row className="g-3">
        <Col xl={8}>
          <Card className="panel">
            <Card.Header>
              <div>
                <h5>Fleet activity</h5>
                <small>Vehicle locations currently reported</small>
              </div>
              <NavLink to="/tracking" className="text-link">
                Open live map <ChevronRight size={14} />
              </NavLink>
            </Card.Header>
            <Card.Body>
              <div className="mini-map">
                <Suspense
                  fallback={
                    <div className="empty">
                      <Spinner size="sm" /> Loading map
                    </div>
                  }
                >
                  <MapView
                    geofences={d.geofences}
                    vehicles={d.vehicles}
                    height="285px"
                  />
                </Suspense>
              </div>
            </Card.Body>
          </Card>
        </Col>
        <Col xl={4}>
          <Card className="panel h-100">
            <Card.Header>
              <div>
                <h5>Geofence distribution</h5>
                <small>Zones by category</small>
              </div>
            </Card.Header>
            <Card.Body>
              <Suspense
                fallback={
                  <div className="empty">
                    <Spinner size="sm" /> Loading chart
                  </div>
                }
              >
                <ZoneChart counts={counts} total={d.geofences.length} />
              </Suspense>
            </Card.Body>
          </Card>
        </Col>
        <Col xl={7}>
          <Card className="panel">
            <Card.Header>
              <div>
                <h5>Recent violations</h5>
                <small>Latest geofence entry and exit events</small>
              </div>
              <NavLink to="/history" className="text-link">
                View history <ChevronRight size={14} />
              </NavLink>
            </Card.Header>
            <Card.Body className="flush">
              <Events
                rows={d.violations.slice(-5).reverse()}
                loading={d.loading}
              />
            </Card.Body>
          </Card>
        </Col>
        <Col xl={5}>
          <Card className="panel">
            <Card.Header>
              <div>
                <h5>Live alerts</h5>
                <small>WebSocket event stream</small>
              </div>
              <Badge bg="success-subtle" text="success">
                LIVE
              </Badge>
            </Card.Header>
            <Card.Body className="flush">
              <Events rows={alerts.slice(0, 5)} loading={false} />
            </Card.Body>
          </Card>
        </Col>
      </Row>
    </>
  );
}
function Events({ rows, loading }) {
  if (loading)
    return (
      <div className="empty">
        <Spinner size="sm" /> Loading events
      </div>
    );
  if (!rows.length)
    return (
      <div className="empty">
        <Activity size={24} />
        <span>No events recorded yet</span>
        <small>Vehicle geofence activity will appear here.</small>
      </div>
    );
  return (
    <div className="event-list">
      {rows.map((x, i) => (
        <div className="event-row" key={x.id || i}>
          <span className={"event-mark " + x.event_type}>
            <MapPin size={15} />
          </span>
          <div className="event-info">
            <b>{x.vehicle_number || x.vehicle_id}</b>
            <span>
              {x.driver_name || "Driver unavailable"} -{" "}
              {x.geofence_name || x.geofence_id} -{" "}
              {x.category?.replace("_", " ")}
            </span>
            <span>
              {x.latitude != null && x.longitude != null
                ? `${x.latitude.toFixed(5)}, ${x.longitude.toFixed(5)}`
                : "Location unavailable"}
            </span>
          </div>
          <Badge bg={x.event_type === "entry" ? "primary" : "secondary"}>
            {x.event_type}
          </Badge>
          <time>{fmt(x.timestamp)}</time>
        </div>
      ))}
    </div>
  );
}
function Tracking({ refresh, notify }) {
  const d = useData(refresh),
    [sel, setSel] = useState(""),
    [point, setPoint] = useState(null),
    [membership, setMembership] = useState([]);
  const selected = d.vehicles.find((v) => v.id === sel);
  useEffect(() => {
    const timer = setInterval(d.load, 5000);
    return () => clearInterval(timer);
  }, []);
  useEffect(() => {
    if (!sel) {
      setMembership([]);
      return;
    }
    api
      .get(`/vehicles/location/${sel}`)
      .then((r) => setMembership(r.data.current_geofences || []))
      .catch((e) => notify(e.response?.data?.error || e.message));
  }, [sel, d.vehicles]);
  async function update(p) {
    if (!sel) {
      notify("Select a vehicle before choosing a map location");
      return;
    }
    try {
      await api.post("/vehicles/location", {
        vehicle_id: sel,
        latitude: p.lat,
        longitude: p.lng,
      });
      setPoint(null);
      d.load();
      notify("Vehicle location updated");
    } catch (e) {
      notify(e.response?.data?.error || e.message);
    }
  }
  return (
    <>
      <Header
        title="Live tracking"
        sub="Monitor vehicle positions and geofence boundaries in real time."
        action={
          <Button className="primary-btn" onClick={() => d.load()}>
            <RefreshCw size={16} /> Refresh map
          </Button>
        }
      />
      {d.error && <ErrorBox error={d.error} />}
      <Card className="panel map-panel">
        <Card.Header>
          <div>
            <h5>Fleet map</h5>
            <small>
              {d.vehicles.filter((v) => v.latitude != null).length} reporting ·{" "}
              {d.geofences.length} zones
            </small>
          </div>
          <Form.Select
            className="select-sm"
            value={sel}
            onChange={(e) => setSel(e.target.value)}
          >
            <option value="">Select vehicle to update</option>
            {d.vehicles.map((v) => (
              <option key={v.id} value={v.id}>
                {v.vehicle_number} · {v.driver_name}
              </option>
            ))}
          </Form.Select>
        </Card.Header>
        <Suspense
          fallback={
            <div className="empty">
              <Spinner size="sm" /> Loading map
            </div>
          }
        >
          <MapView
            geofences={d.geofences}
            vehicles={d.vehicles}
            height="calc(100vh - 290px)"
            onPick={update}
            locationSelectionActive={Boolean(sel)}
            onVehicle={(v) => setSel(v.id)}
          />
        </Suspense>
      </Card>
      <div className="map-hint">
        <MapPin size={15} />{" "}
        {selected
          ? `Click the map to update ${selected.vehicle_number}`
          : "Choose a vehicle, then click the map to set its location."}{" "}
        {point && "Updating…"}
        {selected && (
          <span>
            {" "}
            · Inside:{" "}
            {membership.map((g) => g.name).join(", ") || "No geofences"}
          </span>
        )}
      </div>
    </>
  );
}
function Geofences({ notify, refreshAll }) {
  const [show, setShow] = useState(false),
    [rows, setRows] = useState([]),
    [search, setSearch] = useState(""),
    [cat, setCat] = useState(""),
    [coords, setCoords] = useState(""),
    [drawReset, setDrawReset] = useState(0),
    [editID, setEditID] = useState(""),
    [form, setForm] = useState({
      name: "",
      description: "",
      category: "delivery_zone",
    }),
    [err, setErr] = useState(""),
    [loading, setLoading] = useState(true);
  const onDraw = useCallback(
    (points) => setCoords(points.map((p) => p.join(",")).join(";")),
    [],
  );
  const load = () => {
    setLoading(true);
    setErr("");
    return api
      .get("/geofences")
      .then((x) => setRows(x.data.geofences || []))
      .catch((e) => setErr(e.response?.data?.error || e.message))
      .finally(() => setLoading(false));
  };
  const beginNew = () => {
    setEditID("");
    setForm({ name: "", description: "", category: "delivery_zone" });
    setShow(true);
  };
  const beginEdit = (g) => {
    setEditID(g.id);
    setForm({ name: g.name, description: g.description, category: g.category });
    setCoords(g.coordinates.map((p) => p.join(",")).join(";"));
    setShow(true);
  };
  useEffect(() => {
    load();
  }, []);
  let filtered = rows.filter(
    (g) =>
      g.name.toLowerCase().includes(search.toLowerCase()) &&
      (!cat || g.category === cat),
  );
  async function submit(e) {
    e.preventDefault();
    try {
      let points = coords
        .split(";")
        .filter((s) => s.trim())
        .map((s) => s.split(",").map((v) => Number(v.trim())));
      if (
        points.some((p) => p.length !== 2 || p.some((n) => !Number.isFinite(n)))
      )
        throw new Error("Enter valid latitude,longitude coordinate pairs");
      if (
        points.length &&
        (points[0][0] !== points.at(-1)[0] || points[0][1] !== points.at(-1)[1])
      )
        points.push([...points[0]]);
      if (points.length < 4)
        throw new Error("A polygon must contain at least four points");
      if (editID)
        await api.put("/geofences/" + editID, { ...form, coordinates: points });
      else await api.post("/geofences", { ...form, coordinates: points });
      setShow(false);
      setEditID("");
      setForm({ name: "", description: "", category: "delivery_zone" });
      setCoords("");
      setDrawReset((x) => x + 1);
      load();
      refreshAll();
      notify(editID ? "Geofence updated" : "Geofence created");
    } catch (e) {
      notify(e.response?.data?.error || e.message);
    }
  }
  return (
    <>
      <Header
        title="Geofence management"
        sub="Create and manage the zones that protect your operations."
        action={
          <Button className="primary-btn" onClick={beginNew}>
            <Plus size={17} /> New geofence
          </Button>
        }
      />
      {err && <ErrorBox error={err} />}
      <Row className="g-3">
        <Col xl={8}>
          <Card className="panel">
            <Card.Header>
              <div>
                <h5>Geofence boundaries</h5>
                <small>
                  Use the polygon toolbar to draw, edit or delete the unsaved
                  boundary
                </small>
              </div>
            </Card.Header>
            <Card.Body className="p-0">
              <Suspense
                fallback={
                  <div className="empty">
                    <Spinner size="sm" /> Loading map
                  </div>
                }
              >
                <GeofenceMap
                  geofences={rows}
                  onDraw={onDraw}
                  clearKey={drawReset}
                />
              </Suspense>
            </Card.Body>
          </Card>
        </Col>
        <Col xl={4}>
          <Card className="panel">
            <Card.Header>
              <div>
                <h5>Saved geofences</h5>
                <small>{filtered.length} zones</small>
              </div>
            </Card.Header>
            <Card.Body>
              <InputGroup className="mb-2">
                <InputGroup.Text>
                  <Search size={15} />
                </InputGroup.Text>
                <Form.Control
                  placeholder="Search geofences"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </InputGroup>
              <Form.Select
                className="mb-3"
                value={cat}
                onChange={(e) => setCat(e.target.value)}
              >
                <option value="">All categories</option>
                {Object.keys(colors).map((c) => (
                  <option key={c} value={c}>
                    {c.replace("_", " ")}
                  </option>
                ))}
              </Form.Select>
              <div className="zone-list">
                {filtered.map((g) => (
                  <div className="zone-row" key={g.id}>
                    <i style={{ background: colors[g.category] }} />
                    <div>
                      <b>{g.name}</b>
                      <small>
                        {g.category.replace("_", " ")} · {g.coordinates.length}{" "}
                        points
                      </small>
                    </div>
                    <Button
                      variant="light"
                      className="icon-btn"
                      aria-label={"Edit " + g.name}
                      onClick={() => beginEdit(g)}
                    >
                      <Pencil size={14} />
                    </Button>
                    <Badge bg="success-subtle" text="success">
                      Active
                    </Badge>
                  </div>
                ))}
              </div>
              {loading ? (
                <div className="empty">
                  <Spinner size="sm" /> Loading geofences
                </div>
              ) : (
                !filtered.length && (
                  <div className="empty">
                    <MapPinned size={24} />
                    <span>No geofences yet</span>
                  </div>
                )
              )}
            </Card.Body>
          </Card>
        </Col>
      </Row>
      <Modal show={show} onHide={() => setShow(false)} centered>
        <Form onSubmit={submit}>
          <Modal.Header closeButton>
            <Modal.Title>
              {editID ? "Edit geofence" : "Create geofence"}
            </Modal.Title>
          </Modal.Header>
          <Modal.Body>
            <Form.Group className="mb-3">
              <Form.Label>Name</Form.Label>
              <Form.Control
                required
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
              />
            </Form.Group>
            <Form.Group className="mb-3">
              <Form.Label>Description</Form.Label>
              <Form.Control
                value={form.description}
                onChange={(e) =>
                  setForm({ ...form, description: e.target.value })
                }
              />
            </Form.Group>
            <Form.Group className="mb-3">
              <Form.Label>Category</Form.Label>
              <Form.Select
                value={form.category}
                onChange={(e) => setForm({ ...form, category: e.target.value })}
              >
                {Object.keys(colors).map((c) => (
                  <option value={c} key={c}>
                    {c}
                  </option>
                ))}
              </Form.Select>
            </Form.Group>
            <Form.Group>
              <Form.Label>Closed polygon coordinates</Form.Label>
              <Form.Control
                as="textarea"
                rows={4}
                value={coords}
                onChange={(e) => setCoords(e.target.value)}
              />
              <Form.Text>
                Enter lat,lon pairs separated by semicolons. First and last must
                match; minimum 4 points.
              </Form.Text>
            </Form.Group>
          </Modal.Body>
          <Modal.Footer>
            <Button variant="light" onClick={() => setShow(false)}>
              Cancel
            </Button>
            <Button type="submit" className="primary-btn">
              {editID ? "Save changes" : "Create geofence"}
            </Button>
          </Modal.Footer>
        </Form>
      </Modal>
    </>
  );
}
function Vehicles({ notify, refreshAll }) {
  const d = useData(0),
    [show, setShow] = useState(false),
    [q, setQ] = useState(""),
    [locationStatus, setLocationStatus] = useState(""),
    [details, setDetails] = useState(null),
    [detailLoading, setDetailLoading] = useState(false),
    [form, setForm] = useState({
      vehicle_number: "",
      driver_name: "",
      vehicle_type: "truck",
      phone: "",
    });
  async function submit(e) {
    e.preventDefault();
    try {
      await api.post("/vehicles", form);
      setShow(false);
      setForm({
        vehicle_number: "",
        driver_name: "",
        vehicle_type: "truck",
        phone: "",
      });
      d.load();
      refreshAll();
      notify("Vehicle registered");
    } catch (e) {
      notify(e.response?.data?.error || e.message);
    }
  }
  async function showDetails(v) {
    setDetails({ vehicle: v, current_geofences: [] });
    setDetailLoading(true);
    try {
      const r = await api.get("/vehicles/location/" + v.id);
      setDetails(r.data);
    } catch (e) {
      notify(e.response?.data?.error || e.message);
    } finally {
      setDetailLoading(false);
    }
  }
  const rows = d.vehicles.filter(
    (v) =>
      `${v.vehicle_number} ${v.driver_name} ${v.vehicle_type}`
        .toLowerCase()
        .includes(q.toLowerCase()) &&
      (!locationStatus ||
        (locationStatus === "reported"
          ? v.latitude != null
          : v.latitude == null)),
  );
  return (
    <>
      <Header
        title="Vehicle management"
        sub="Keep your fleet roster, drivers and latest positions up to date."
        action={
          <Button className="primary-btn" onClick={() => setShow(true)}>
            <Plus size={17} /> Register vehicle
          </Button>
        }
      />
      {d.error && <ErrorBox error={d.error} />}
      <Card className="panel">
        <Card.Header>
          <div>
            <h5>Registered vehicles</h5>
            <small>{rows.length} vehicles in fleet</small>
          </div>
          <InputGroup className="search-box">
            <InputGroup.Text>
              <Search size={15} />
            </InputGroup.Text>
            <Form.Control
              placeholder="Search vehicles"
              value={q}
              onChange={(e) => setQ(e.target.value)}
            />
          </InputGroup>
          <Form.Select
            aria-label="Filter by location status"
            className="select-sm"
            value={locationStatus}
            onChange={(e) => setLocationStatus(e.target.value)}
          >
            <option value="">All location states</option>
            <option value="reported">Location reported</option>
            <option value="none">No location yet</option>
          </Form.Select>
        </Card.Header>
        <Card.Body className="flush">
          <Table responsive hover className="fleet-table">
            <thead>
              <tr>
                <th>VEHICLE</th>
                <th>DRIVER</th>
                <th>TYPE</th>
                <th>STATUS</th>
                <th>PHONE</th>
                <th>LATEST LOCATION</th>
                <th>UPDATED</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((v) => (
                <tr key={v.id}>
                  <td>
                    <span className="vehicle-avatar">
                      <Truck size={17} />
                    </span>
                    <Button
                      variant="link"
                      className="vehicle-detail-link"
                      onClick={() => showDetails(v)}
                    >
                      {v.vehicle_number}
                    </Button>
                  </td>
                  <td>{v.driver_name}</td>
                  <td>
                    <Badge className="soft-badge">{v.vehicle_type}</Badge>
                  </td>
                  <td>
                    <Badge
                      bg={v.latitude != null ? "success-subtle" : "secondary"}
                      text={v.latitude != null ? "success" : "secondary"}
                    >
                      {v.latitude != null ? "Located" : "No location"}
                    </Badge>
                  </td>
                  <td>{v.phone}</td>
                  <td>
                    {v.latitude != null
                      ? `${v.latitude.toFixed(5)}, ${v.longitude.toFixed(5)}`
                      : "—"}
                  </td>
                  <td>{fmt(v.updated_at)}</td>
                </tr>
              ))}
            </tbody>
          </Table>
          {d.loading ? (
            <div className="empty">
              <Spinner size="sm" /> Loading vehicles
            </div>
          ) : (
            !rows.length && (
              <div className="empty">
                <Truck size={25} />
                <span>No vehicles registered</span>
                <small>Register your first vehicle to start tracking.</small>
              </div>
            )
          )}
        </Card.Body>
      </Card>
      <Modal show={show} onHide={() => setShow(false)} centered>
        <Form onSubmit={submit}>
          <Modal.Header closeButton>
            <Modal.Title>Register vehicle</Modal.Title>
          </Modal.Header>
          <Modal.Body>
            {[
              ["vehicle_number", "Vehicle number"],
              ["driver_name", "Driver name"],
              ["vehicle_type", "Vehicle type"],
              ["phone", "Phone"],
            ].map(([k, n]) => (
              <Form.Group className="mb-3" key={k}>
                <Form.Label>{n}</Form.Label>
                <Form.Control
                  required
                  value={form[k]}
                  onChange={(e) => setForm({ ...form, [k]: e.target.value })}
                />
              </Form.Group>
            ))}
          </Modal.Body>
          <Modal.Footer>
            <Button variant="light" onClick={() => setShow(false)}>
              Cancel
            </Button>
            <Button type="submit" className="primary-btn">
              Register vehicle
            </Button>
          </Modal.Footer>
        </Form>
      </Modal>
      <Modal show={!!details} onHide={() => setDetails(null)} centered>
        <Modal.Header closeButton>
          <Modal.Title>Vehicle details</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          {detailLoading ? (
            <div className="empty">
              <Spinner size="sm" /> Loading vehicle details
            </div>
          ) : (
            details && (
              <>
                <div className="detail-grid">
                  <div>
                    <small>VEHICLE</small>
                    <b>{details.vehicle.vehicle_number}</b>
                  </div>
                  <div>
                    <small>DRIVER</small>
                    <b>{details.vehicle.driver_name}</b>
                  </div>
                  <div>
                    <small>TYPE</small>
                    <b>{details.vehicle.vehicle_type}</b>
                  </div>
                  <div>
                    <small>PHONE</small>
                    <b>{details.vehicle.phone}</b>
                  </div>
                  <div>
                    <small>LOCATION</small>
                    <b>
                      {details.vehicle.latitude == null
                        ? "Not reported"
                        : `${details.vehicle.latitude.toFixed(5)}, ${details.vehicle.longitude.toFixed(5)}`}
                    </b>
                  </div>
                  <div>
                    <small>LAST UPDATE</small>
                    <b>{fmt(details.vehicle.updated_at)}</b>
                  </div>
                </div>
                <div className="section-title">Current geofence membership</div>
                {details.current_geofences.length ? (
                  details.current_geofences.map((g) => (
                    <Badge
                      key={g.id}
                      className="me-2 mb-2"
                      bg="primary-subtle"
                      text="primary"
                    >
                      {g.name}
                    </Badge>
                  ))
                ) : (
                  <span className="muted">No active geofence membership</span>
                )}
              </>
            )
          )}
        </Modal.Body>
        <Modal.Footer>
          <Button variant="light" onClick={() => setDetails(null)}>
            Close
          </Button>
        </Modal.Footer>
      </Modal>
    </>
  );
}
function Rules({ notify, refresh }) {
  const d = useData(refresh),
    [form, setForm] = useState({
      geofence_id: "",
      vehicle_id: "",
      event_type: "entry",
    });
  async function submit(e) {
    e.preventDefault();
    try {
      await api.post("/alerts/configure", form);
      notify("Alert rule configured");
      d.load();
    } catch (e) {
      notify(e.response?.data?.error || e.message);
    }
  }
  return (
    <>
      <Header
        title="Alert configuration"
        sub="Choose which geofence events should notify your operations team."
      />
      <Row className="g-3">
        <Col lg={5}>
          <Card className="panel">
            <Card.Header>
              <div>
                <h5>New alert rule</h5>
                <small>Rules can target one vehicle or the whole fleet</small>
              </div>
            </Card.Header>
            <Card.Body>
              <Form onSubmit={submit}>
                <Form.Group className="mb-3">
                  <Form.Label>Geofence</Form.Label>
                  <Form.Select
                    required
                    value={form.geofence_id}
                    onChange={(e) =>
                      setForm({ ...form, geofence_id: e.target.value })
                    }
                  >
                    <option value="">Select geofence</option>
                    {d.geofences.map((g) => (
                      <option key={g.id} value={g.id}>
                        {g.name}
                      </option>
                    ))}
                  </Form.Select>
                </Form.Group>
                <Form.Group className="mb-3">
                  <Form.Label>Vehicle scope</Form.Label>
                  <Form.Select
                    value={form.vehicle_id}
                    onChange={(e) =>
                      setForm({ ...form, vehicle_id: e.target.value })
                    }
                  >
                    <option value="">All vehicles</option>
                    {d.vehicles.map((v) => (
                      <option key={v.id} value={v.id}>
                        {v.vehicle_number} · {v.driver_name}
                      </option>
                    ))}
                  </Form.Select>
                </Form.Group>
                <Form.Group className="mb-4">
                  <Form.Label>Trigger event</Form.Label>
                  <div className="radio-options">
                    {["entry", "exit", "both"].map((e) => (
                      <label
                        className={form.event_type === e ? "chosen" : ""}
                        key={e}
                      >
                        <Form.Check
                          type="radio"
                          name="event"
                          checked={form.event_type === e}
                          onChange={() => setForm({ ...form, event_type: e })}
                        />
                        <span>{e === "both" ? "Entry + exit" : e}</span>
                      </label>
                    ))}
                  </div>
                </Form.Group>
                <Button className="primary-btn w-100" type="submit">
                  <Plus size={16} /> Save alert rule
                </Button>
              </Form>
            </Card.Body>
          </Card>
        </Col>
        <Col lg={7}>
          <Card className="panel">
            <Card.Header>
              <div>
                <h5>Configured rules</h5>
                <small>{d.alerts.length} alert rules</small>
              </div>
            </Card.Header>
            <Card.Body className="flush">
              <div className="event-list">
                {d.alerts.map((x) => (
                  <div className="event-row" key={x.id}>
                    <span className="event-mark entry">
                      <Bell size={15} />
                    </span>
                    <div className="event-info">
                      <b>
                        {d.geofences.find((g) => g.id === x.geofence_id)
                          ?.name || x.geofence_id}
                      </b>
                      <span>
                        {x.vehicle_id
                          ? d.vehicles.find((v) => v.id === x.vehicle_id)
                              ?.vehicle_number || x.vehicle_id
                          : "All vehicles"}
                      </span>
                    </div>
                    <Badge bg="primary-subtle" text="primary">
                      {x.event_type}
                    </Badge>
                    <Badge
                      bg={x.active ? "success-subtle" : "secondary"}
                      text={x.active ? "success" : "secondary"}
                    >
                      {x.active ? "Active" : "Paused"}
                    </Badge>
                  </div>
                ))}
              </div>
              {d.loading ? (
                <div className="empty">
                  <Spinner size="sm" /> Loading alert rules
                </div>
              ) : (
                !d.alerts.length && (
                  <div className="empty">
                    <Bell size={24} />
                    <span>No alert rules configured</span>
                  </div>
                )
              )}
            </Card.Body>
          </Card>
        </Col>
      </Row>
    </>
  );
}
function AlertPage({ alerts, refresh }) {
  const d = useData(refresh);
  return (
    <>
      <Header
        title="Live alerts"
        sub="Real-time entry and exit events received from connected vehicles."
        action={
          <span className="ws-status good">
            <i /> Streaming live
          </span>
        }
      />
      <Card className="panel">
        <Card.Header>
          <div>
            <h5>Recent alert stream</h5>
            <small>{alerts.length} events received this session</small>
          </div>
          <Badge bg="success-subtle" text="success">
            WEBSOCKET
          </Badge>
        </Card.Header>
        <Card.Body className="flush">
          {alerts.length ? (
            <Events rows={alerts} />
          ) : (
            <div className="empty tall">
              <Radio size={28} />
              <span>Waiting for live events</span>
              <small>
                Alert rules will stream matching vehicle events here.
              </small>
            </div>
          )}
        </Card.Body>
      </Card>
      <div className="section-title">Persisted history</div>
      <Card className="panel">
        <Card.Body className="flush">
          <Events
            rows={d.violations.slice(-10).reverse()}
            loading={d.loading}
          />
        </Card.Body>
      </Card>
    </>
  );
}
function HistoryPage({ refresh }) {
  const d = useData(refresh),
    [v, setV] = useState(""),
    [g, setG] = useState(""),
    [from, setFrom] = useState(""),
    [to, setTo] = useState(""),
    [page, setPage] = useState(0),
    [records, setRecords] = useState([]),
    [total, setTotal] = useState(0),
    [loading, setLoading] = useState(true),
    [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    setLoading(true);
    const params = { limit: 10, offset: page * 10 };
    if (v) params.vehicle_id = v;
    if (g) params.geofence_id = g;
    if (from) params.from = new Date(from + "T00:00:00").toISOString();
    if (to) params.to = new Date(to + "T23:59:59.999").toISOString();
    api
      .get("/violations/history", { params })
      .then((r) => {
        if (active) {
          setRecords(r.data.violations || []);
          setTotal(r.data.total || 0);
          setError("");
        }
      })
      .catch((e) => {
        if (active) setError(e.response?.data?.error || e.message);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [v, g, from, to, page, refresh]);
  const pages = Math.max(1, Math.ceil(total / 10));
  return (
    <>
      <Header
        title="Violation history"
        sub="Review vehicle entry and exit activity across monitored zones."
      />
      {error && <ErrorBox error={error} />}
      <Card className="panel mb-3">
        <Card.Body>
          <Row className="g-3">
            <Col md={3}>
              <Form.Label className="filter-label">VEHICLE</Form.Label>
              <Form.Select
                value={v}
                onChange={(e) => {
                  setV(e.target.value);
                  setPage(0);
                }}
              >
                <option value="">All vehicles</option>
                {d.vehicles.map((x) => (
                  <option value={x.id} key={x.id}>
                    {x.vehicle_number}
                  </option>
                ))}
              </Form.Select>
            </Col>
            <Col md={3}>
              <Form.Label className="filter-label">GEOFENCE</Form.Label>
              <Form.Select
                value={g}
                onChange={(e) => {
                  setG(e.target.value);
                  setPage(0);
                }}
              >
                <option value="">All geofences</option>
                {d.geofences.map((x) => (
                  <option value={x.id} key={x.id}>
                    {x.name}
                  </option>
                ))}
              </Form.Select>
            </Col>
            <Col md={3}>
              <Form.Label className="filter-label">FROM</Form.Label>
              <Form.Control
                type="date"
                value={from}
                onChange={(e) => {
                  setFrom(e.target.value);
                  setPage(0);
                }}
              />
            </Col>
            <Col md={3}>
              <Form.Label className="filter-label">TO</Form.Label>
              <Form.Control
                type="date"
                value={to}
                onChange={(e) => {
                  setTo(e.target.value);
                  setPage(0);
                }}
              />
            </Col>
          </Row>
        </Card.Body>
      </Card>
      <Card className="panel">
        <Card.Header>
          <div>
            <h5>Geofence events</h5>
            <small>{total} records found</small>
          </div>
          <span className="muted">Max 500 per request</span>
        </Card.Header>
        <Card.Body className="flush">
          {loading ? (
            <div className="empty">
              <Spinner size="sm" /> Loading violations
            </div>
          ) : records.length ? (
            <Table responsive hover className="fleet-table">
              <thead>
                <tr>
                  <th>EVENT</th>
                  <th>VEHICLE</th>
                  <th>DRIVER</th>
                  <th>GEOFENCE</th>
                  <th>COORDINATES</th>
                  <th>TIMESTAMP</th>
                </tr>
              </thead>
              <tbody>
                {records.map((x) => (
                  <tr key={x.id}>
                    <td>
                      <Badge
                        bg={x.event_type === "entry" ? "primary" : "secondary"}
                      >
                        {x.event_type}
                      </Badge>
                    </td>
                    <td>
                      <b>{x.vehicle_number}</b>
                    </td>
                    <td>{x.driver_name}</td>
                    <td>{x.geofence_name}</td>
                    <td>
                      {x.latitude.toFixed(5)}, {x.longitude.toFixed(5)}
                    </td>
                    <td>{fmt(x.timestamp)}</td>
                  </tr>
                ))}
              </tbody>
            </Table>
          ) : (
            <div className="empty">
              <History size={24} />
              <span>No matching events</span>
              <small>
                Events appear after vehicles cross configured geofence
                boundaries.
              </small>
            </div>
          )}
        </Card.Body>
        <Card.Footer className="pager">
          <span>
            Showing {total ? page * 10 + 1 : 0} -{" "}
            {Math.min((page + 1) * 10, total)} of {total}
          </span>
          <div>
            <Button
              variant="light"
              disabled={!page}
              onClick={() => setPage(page - 1)}
            >
              Previous
            </Button>
            <span>
              Page {page + 1} of {pages}
            </span>
            <Button
              variant="light"
              disabled={page + 1 >= pages}
              onClick={() => setPage(page + 1)}
            >
              Next
            </Button>
          </div>
        </Card.Footer>
      </Card>
    </>
  );
}
function ErrorBox({ error }) {
  return (
    <div className="api-error">
      <AlertTriangle size={17} />
      <div>
        <b>Unable to load data</b>
        <span>{error}</span>
      </div>
    </div>
  );
}
createRoot(document.getElementById("root")).render(
  <BrowserRouter>
    <App />
  </BrowserRouter>,
);
