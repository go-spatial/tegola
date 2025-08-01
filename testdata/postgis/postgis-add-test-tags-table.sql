DROP TABLE IF EXISTS test_tags_table;

-- Create table to store polygon geometries in SRID 4326 with an explicit UUID column,
-- an hstore column, extra text, and extra integer columns.
CREATE TABLE test_tags_table (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL,
    name TEXT,
    geom GEOMETRY(Polygon, 4326),
    properties hstore,
    extra_text TEXT,
    extra_int INT
);

-- Create a spatial index on the geometry column for improved spatial query performance.
CREATE INDEX idx_test_tags_table_geom ON test_tags_table USING GIST (geom);

-- Insert 5 example polygons within the Berlin area with sample hstore data,
-- extra columns, and explicitly provided UUID values.
INSERT INTO test_tags_table (uuid, name, geom, properties, extra_text, extra_int) VALUES 
('550e8400-e29b-41d4-a716-446655440000', 
 'Polygon A', 
 ST_SetSRID(ST_GeomFromText('POLYGON((13.4045 52.5195, 13.4055 52.5195, 13.4055 52.5205, 13.4045 52.5205, 13.4045 52.5195))'), 4326),
 '"count"=>"42", "enabled"=>"true", "price"=>"19.99", "description"=>"example polygon A"',
 'Additional info A', 100
),

('550e8400-e29b-41d4-a716-446655440001', 
 'Polygon B', 
 ST_SetSRID(ST_GeomFromText('POLYGON((13.4065 52.5180, 13.4075 52.5180, 13.4075 52.5190, 13.4065 52.5190, 13.4065 52.5180))'), 4326),
 '"count"=>"7", "enabled"=>"false", "price"=>"5.50", "description"=>"example polygon B"',
 'Additional info B', 200
),

('550e8400-e29b-41d4-a716-446655440002', 
 'Polygon C', 
 ST_SetSRID(ST_GeomFromText('POLYGON((13.4025 52.5205, 13.4035 52.5205, 13.4035 52.5215, 13.4025 52.5215, 13.4025 52.5205))'), 4326),
 '"count"=>"15", "enabled"=>"true", "price"=>"12.00", "description"=>"example polygon C"',
 'Additional info C', 300
),

('550e8400-e29b-41d4-a716-446655440003', 
 'Polygon D', 
 ST_SetSRID(ST_GeomFromText('POLYGON((13.4085 52.5215, 13.4095 52.5215, 13.4095 52.5225, 13.4085 52.5225, 13.4085 52.5215))'), 4326),
 '"count"=>"23", "enabled"=>"true", "price"=>"7.25", "description"=>"example polygon D"',
 'Additional info D', 400
),

('550e8400-e29b-41d4-a716-446655440004', 
 'Polygon E', 
 ST_SetSRID(ST_GeomFromText('POLYGON((13.4045 52.5225, 13.4055 52.5225, 13.4055 52.5235, 13.4045 52.5235, 13.4045 52.5225))'), 4326),
 '"count"=>"99", "enabled"=>"false", "price"=>"20.00", "description"=>"example polygon E"',
 'Additional info E', 500
);
