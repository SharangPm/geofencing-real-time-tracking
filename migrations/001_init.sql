CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS geofences (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL CHECK (length(trim(name)) > 0),
    description text NOT NULL DEFAULT '',
    category text NOT NULL CHECK (category IN ('delivery_zone','restricted_zone','toll_zone','customer_area')),
    coordinates jsonb NOT NULL CHECK (jsonb_typeof(coordinates) = 'array' AND jsonb_array_length(coordinates) >= 4),
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS geofences_category_active_idx ON geofences(category, active);

CREATE TABLE IF NOT EXISTS vehicles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_number text NOT NULL,
    driver_name text NOT NULL CHECK (length(trim(driver_name)) > 0),
    vehicle_type text NOT NULL CHECK (length(trim(vehicle_type)) > 0),
    phone text NOT NULL CHECK (length(trim(phone)) > 0),
    latitude double precision CHECK (latitude BETWEEN -90 AND 90),
    longitude double precision CHECK (longitude BETWEEN -180 AND 180),
    updated_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((latitude IS NULL) = (longitude IS NULL))
);
CREATE UNIQUE INDEX IF NOT EXISTS vehicles_number_ci_uq ON vehicles(lower(vehicle_number));
CREATE INDEX IF NOT EXISTS vehicles_updated_at_idx ON vehicles(updated_at DESC);

CREATE TABLE IF NOT EXISTS vehicle_locations (
    id bigserial PRIMARY KEY,
    vehicle_id uuid NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    latitude double precision NOT NULL CHECK (latitude BETWEEN -90 AND 90),
    longitude double precision NOT NULL CHECK (longitude BETWEEN -180 AND 180),
    recorded_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS vehicle_locations_vehicle_time_idx ON vehicle_locations(vehicle_id, recorded_at DESC);

CREATE TABLE IF NOT EXISTS vehicle_geofence_states (
    vehicle_id uuid NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    geofence_id uuid NOT NULL REFERENCES geofences(id) ON DELETE CASCADE,
    is_inside boolean NOT NULL DEFAULT false,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(vehicle_id, geofence_id)
);
CREATE INDEX IF NOT EXISTS vehicle_geofence_inside_idx ON vehicle_geofence_states(geofence_id, vehicle_id) WHERE is_inside;

CREATE TABLE IF NOT EXISTS alert_configurations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    geofence_id uuid NOT NULL REFERENCES geofences(id) ON DELETE CASCADE,
    vehicle_id uuid REFERENCES vehicles(id) ON DELETE CASCADE,
    event_type text NOT NULL CHECK (event_type IN ('entry','exit','both')),
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS alert_configurations_match_idx ON alert_configurations(geofence_id, vehicle_id, active, event_type);
CREATE UNIQUE INDEX IF NOT EXISTS alert_configurations_unique_idx ON alert_configurations(geofence_id, COALESCE(vehicle_id, '00000000-0000-0000-0000-000000000000'::uuid), event_type);

CREATE TABLE IF NOT EXISTS violations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id uuid NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    geofence_id uuid NOT NULL REFERENCES geofences(id) ON DELETE CASCADE,
    event_type text NOT NULL CHECK (event_type IN ('entry','exit')),
    latitude double precision NOT NULL CHECK (latitude BETWEEN -90 AND 90),
    longitude double precision NOT NULL CHECK (longitude BETWEEN -180 AND 180),
    event_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS violations_vehicle_time_idx ON violations(vehicle_id, event_at DESC);
CREATE INDEX IF NOT EXISTS violations_geofence_time_idx ON violations(geofence_id, event_at DESC);
CREATE INDEX IF NOT EXISTS violations_time_idx ON violations(event_at DESC);
