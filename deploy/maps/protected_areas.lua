-- OSM flex importer captures protected areas, typed tourism, heritage, outdoor routes, and water sports while preserving names, IDs, classifications, and source tags.
local tables = {}

tables.protected_areas = osm2pgsql.define_area_table('protected_areas', {
    { column = 'name', type = 'text' },
    { column = 'boundary', type = 'text' },
    { column = 'leisure', type = 'text' },
    { column = 'protect_class', type = 'text' },
    { column = 'protection_title', type = 'text' },
    { column = 'operator', type = 'text' },
    { column = 'designation', type = 'text' },
    { column = 'tags', type = 'jsonb' },
    { column = 'geom', type = 'multipolygon', projection = 4326, not_null = true },
})

tables.tourism_pois = osm2pgsql.define_node_table('tourism_pois', {
    { column = 'name', type = 'text' },
    { column = 'feature_class', type = 'text', not_null = true },
    { column = 'tourism', type = 'text' },
    { column = 'amenity', type = 'text' },
    { column = 'natural', type = 'text' },
    { column = 'leisure', type = 'text' },
    { column = 'highway', type = 'text' },
    { column = 'tags', type = 'jsonb' },
    { column = 'geom', type = 'point', projection = 4326, not_null = true },
})

tables.tourism_areas = osm2pgsql.define_area_table('tourism_areas', {
    { column = 'name', type = 'text' },
    { column = 'feature_class', type = 'text', not_null = true },
    { column = 'tourism', type = 'text' },
    { column = 'amenity', type = 'text' },
    { column = 'natural', type = 'text' },
    { column = 'leisure', type = 'text' },
    { column = 'highway', type = 'text' },
    { column = 'tags', type = 'jsonb' },
    { column = 'geom', type = 'multipolygon', projection = 4326, not_null = true },
})

tables.heritage_pois = osm2pgsql.define_node_table('heritage_pois', {
    { column = 'name', type = 'text' },
    { column = 'feature_class', type = 'text', not_null = true },
    { column = 'heritage_status', type = 'text' },
    { column = 'heritage_type', type = 'text' },
    { column = 'historical_period', type = 'text' },
    { column = 'civilization', type = 'text' },
    { column = 'designation', type = 'text' },
    { column = 'tags', type = 'jsonb' },
    { column = 'geom', type = 'point', projection = 4326, not_null = true },
})

tables.heritage_areas = osm2pgsql.define_area_table('heritage_areas', {
    { column = 'name', type = 'text' },
    { column = 'feature_class', type = 'text', not_null = true },
    { column = 'heritage_status', type = 'text' },
    { column = 'heritage_type', type = 'text' },
    { column = 'historical_period', type = 'text' },
    { column = 'civilization', type = 'text' },
    { column = 'designation', type = 'text' },
    { column = 'tags', type = 'jsonb' },
    { column = 'geom', type = 'multipolygon', projection = 4326, not_null = true },
})

tables.outdoor_routes = osm2pgsql.define_relation_table('outdoor_routes', {
    { column = 'name', type = 'text' },
    { column = 'feature_class', type = 'text', not_null = true },
    { column = 'route_ref', type = 'text' },
    { column = 'operator', type = 'text' },
    { column = 'network', type = 'text' },
    { column = 'difficulty', type = 'text' },
    { column = 'surface', type = 'text' },
    { column = 'tags', type = 'jsonb' },
    { column = 'geom', type = 'multilinestring', projection = 4326, not_null = true },
})

tables.water_sport_pois = osm2pgsql.define_node_table('water_sport_pois', {
    { column = 'name', type = 'text' },
    { column = 'feature_class', type = 'text', not_null = true },
    { column = 'sport', type = 'text' },
    { column = 'dive_type', type = 'text' },
    { column = 'depth_m', type = 'real' },
    { column = 'tags', type = 'jsonb' },
    { column = 'geom', type = 'point', projection = 4326, not_null = true },
})

local function is_protected(tags)
    return tags.boundary == 'protected_area'
        or tags.boundary == 'national_park'
        or tags.leisure == 'nature_reserve'
        or tags.protect_class ~= nil
end

local function tourism_class(tags)
    if tags.highway == 'trailhead' then return 'trailhead' end
    if tags.leisure == 'marina' then return 'marina' end
    if tags.amenity == 'ferry_terminal' then return 'ferry_terminal' end
    if tags.harbour == 'yes' or tags.leisure == 'slipway' then return 'harbour' end
    if tags.tourism == 'viewpoint' then return 'viewpoint' end
    if tags.tourism == 'attraction' then return 'attraction' end
    if tags.tourism == 'information' then return 'visitor_information' end
    if tags.natural == 'waterfall' then return 'waterfall' end
    if tags.natural == 'peak' then return 'peak' end
    if tags.natural == 'volcano' then return 'volcano' end
    if tags.natural == 'cave_entrance' then return 'cave' end
    if tags.natural == 'hot_spring' then return 'hot_spring' end
    if tags.natural == 'glacier' then return 'glacier' end
    if tags.natural == 'gorge' then return 'gorge' end
    if tags.natural == 'canyon' then return 'canyon' end
    if tags.natural == 'valley' then return 'valley' end
    if tags.natural == 'reef' then return 'reef' end
    if tags.natural == 'beach' then return 'beach' end
    return nil
end

local function heritage_class(tags)
    if tags.historic == nil and tags.heritage == nil then return nil end
    if tags.historic == 'archaeological_site' then return 'archaeological_site' end
    if tags.historic == 'castle' then return 'castle' end
    if tags.historic == 'fort' then return 'fort' end
    if tags.historic == 'tower' then return 'tower' end
    if tags.historic == 'citywalls' then return 'historic_wall' end
    if tags.historic == 'building' then return 'historic_building' end
    if tags.historic == 'ruins' then return 'ruins' end
    if tags.historic == 'tomb' then return 'tomb' end
    if tags.historic == 'memorial' then return 'memorial' end
    if tags.historic == 'monument' then return 'ancient_monument' end
    if tags.historic == 'battlefield' then return 'battlefield' end
    if tags.historic == 'ship' then return 'shipwreck' end
    if tags.historic == 'wayside_shrine' or tags.historic == 'church' or tags.historic == 'mosque' or tags.historic == 'temple' then return 'historic_religious_site' end
    if tags.heritage == '1' or tags.heritage == '2' then return 'heritage_landscape' end
    return nil
end

local function route_class(tags)
    if tags.type ~= 'route' then return nil end
    if tags.route == 'hiking' then return 'hiking_trail' end
    if tags.route == 'foot' or tags.route == 'walking' then return 'walking_route' end
    if tags.route == 'mtb' or tags.route == 'bicycle' then return 'mountain_bike_route' end
    return nil
end

local function water_sport_class(tags)
    if tags.sport == 'scuba_diving' and tags['scuba_diving:divespot'] == 'yes' then return 'scuba_diving_site' end
    if tags.amenity == 'dive_centre' or tags.shop == 'scuba_diving' then return 'dive_centre' end
    if tags.sport == 'scuba_diving' then return 'scuba_diving_site' end
    if tags.sport == 'snorkeling' or tags.sport == 'snorkelling' then return 'snorkelling_site' end
    if tags.sport == 'surfing' then return 'surfing_site' end
    if tags.sport == 'kayaking' then return 'kayaking_site' end
    if tags.sport == 'rafting' then return 'rafting_site' end
    return nil
end

local function add_protected(object, geom)
    if geom:is_null() then return end
    tables.protected_areas:insert({
        name = object.tags.name,
        boundary = object.tags.boundary,
        leisure = object.tags.leisure,
        protect_class = object.tags.protect_class,
        protection_title = object.tags.protection_title,
        operator = object.tags.operator,
        designation = object.tags.designation,
        tags = object.tags,
        geom = geom,
    })
end

local function add_tourism_point(object, geom)
    local feature_class = tourism_class(object.tags)
    if geom:is_null() or feature_class == nil then return end
    tables.tourism_pois:insert({
        name = object.tags.name,
        feature_class = feature_class,
        tourism = object.tags.tourism,
        amenity = object.tags.amenity,
        natural = object.tags.natural,
        leisure = object.tags.leisure,
        highway = object.tags.highway,
        tags = object.tags,
        geom = geom,
    })
end

local function add_tourism_area(object, geom)
    local feature_class = tourism_class(object.tags)
    if geom:is_null() or feature_class == nil then return end
    tables.tourism_areas:insert({
        name = object.tags.name,
        feature_class = feature_class,
        tourism = object.tags.tourism,
        amenity = object.tags.amenity,
        natural = object.tags.natural,
        leisure = object.tags.leisure,
        highway = object.tags.highway,
        tags = object.tags,
        geom = geom,
    })
end

local function add_heritage_point(object, geom)
    local feature_class = heritage_class(object.tags)
    if geom:is_null() or feature_class == nil then return end
    tables.heritage_pois:insert({
        name = object.tags.name,
        feature_class = feature_class,
        heritage_status = object.tags.heritage_status,
        heritage_type = object.tags.historic,
        historical_period = object.tags.start_date,
        civilization = object.tags.civilization,
        designation = object.tags.designation,
        tags = object.tags,
        geom = geom,
    })
end

local function add_heritage_area(object, geom)
    local feature_class = heritage_class(object.tags)
    if geom:is_null() or feature_class == nil then return end
    tables.heritage_areas:insert({
        name = object.tags.name,
        feature_class = feature_class,
        heritage_status = object.tags.heritage_status,
        heritage_type = object.tags.historic,
        historical_period = object.tags.start_date,
        civilization = object.tags.civilization,
        designation = object.tags.designation,
        tags = object.tags,
        geom = geom,
    })
end

local function add_water_sport_point(object, geom)
    local feature_class = water_sport_class(object.tags)
    if geom:is_null() or feature_class == nil then return end
    local depth = tonumber(object.tags.depth)
    tables.water_sport_pois:insert({
        name = object.tags.name,
        feature_class = feature_class,
        sport = object.tags.sport,
        dive_type = object.tags.scuba_diving_type,
        depth_m = depth,
        tags = object.tags,
        geom = geom,
    })
end

function osm2pgsql.process_node(object)
    add_tourism_point(object, object:as_point())
    add_heritage_point(object, object:as_point())
    add_water_sport_point(object, object:as_point())
end

function osm2pgsql.process_way(object)
    if object.is_closed then
        if is_protected(object.tags) then add_protected(object, object:as_polygon()) end
        add_tourism_area(object, object:as_polygon())
        add_heritage_area(object, object:as_polygon())
    end
end

function osm2pgsql.process_relation(object)
    local relation_type = object.tags.type
    if (relation_type == 'multipolygon' or relation_type == 'boundary') and is_protected(object.tags) then
        add_protected(object, object:as_multipolygon())
    end
    local route = route_class(object.tags)
    if route ~= nil and object.tags.name ~= nil then
        local geom = object:as_multilinestring()
        if not geom:is_null() then
            tables.outdoor_routes:insert({
                name = object.tags.name,
                feature_class = route,
                route_ref = object.tags.ref,
                operator = object.tags.operator,
                network = object.tags.network,
                difficulty = object.tags.sac_scale,
                surface = object.tags.surface,
                tags = object.tags,
                geom = geom,
            })
        end
    end
    if relation_type == 'multipolygon' then add_heritage_area(object, object:as_multipolygon()) end
end
