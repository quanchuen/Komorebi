-- osm2pgsql flex style for cyclist-map
-- Targets: Tokyo/Kanto region
-- Captures cycling-relevant tags for roads, infrastructure, and venues

local tables = {}

-- Roads and paths (line geometry)
tables.roads = osm2pgsql.define_way_table('roads', {
    { column = 'name',         type = 'text' },
    { column = 'name_en',      type = 'text' },
    { column = 'highway',      type = 'text' },
    { column = 'cycleway',     type = 'text' },
    { column = 'cycleway_left',  type = 'text' },
    { column = 'cycleway_right', type = 'text' },
    { column = 'bicycle',      type = 'text' },
    { column = 'surface',      type = 'text' },
    { column = 'smoothness',   type = 'text' },
    { column = 'maxspeed',     type = 'text' },
    { column = 'oneway',       type = 'text' },
    { column = 'lit',          type = 'text' },
    { column = 'width',        type = 'text' },
    { column = 'lanes',        type = 'text' },
    { column = 'tags',         type = 'jsonb' },
    { column = 'geom',         type = 'linestring', projection = 4326 },
}, { schema = 'osm' })

-- Points of interest: venues, signals, etc. (node geometry)
tables.pois = osm2pgsql.define_node_table('pois', {
    { column = 'name',      type = 'text' },
    { column = 'name_en',   type = 'text' },
    { column = 'amenity',   type = 'text' },
    { column = 'shop',      type = 'text' },
    { column = 'leisure',   type = 'text' },
    { column = 'highway',   type = 'text' },
    { column = 'tourism',   type = 'text' },
    { column = 'brand',     type = 'text' },
    { column = 'operator',  type = 'text' },
    { column = 'opening_hours', type = 'text' },
    { column = 'phone',     type = 'text' },
    { column = 'website',   type = 'text' },
    { column = 'tags',      type = 'jsonb' },
    { column = 'geom',      type = 'point', projection = 4326 },
}, { schema = 'osm' })

-- Land use and green areas (polygon geometry)
tables.landuse = osm2pgsql.define_area_table('landuse', {
    { column = 'name',      type = 'text' },
    { column = 'landuse',   type = 'text' },
    { column = 'leisure',   type = 'text' },
    { column = 'natural_',  type = 'text' },
    { column = 'tags',      type = 'jsonb' },
    { column = 'geom',      type = 'geometry', projection = 4326 },
}, { schema = 'osm' })

-- Highway-class filter: only import road types relevant to cycling
local highway_keep = {
    trunk = true,
    trunk_link = true,
    primary = true,
    primary_link = true,
    secondary = true,
    secondary_link = true,
    tertiary = true,
    tertiary_link = true,
    unclassified = true,
    residential = true,
    service = true,
    living_street = true,
    pedestrian = true,
    track = true,
    path = true,
    cycleway = true,
    footway = true,
}

-- Amenity/shop types to capture as POIs
local poi_amenity = {
    cafe = true,
    restaurant = true,
    fast_food = true,
    toilets = true,
    drinking_water = true,
    vending_machine = true,
    atm = true,
    bicycle_parking = true,
    bicycle_repair_station = true,
    place_of_worship = true,
}

local poi_shop = {
    convenience = true,
    supermarket = true,
    bicycle = true,
}

local poi_leisure = {
    park = true,
    hot_spring = true,
}

local poi_highway = {
    traffic_signals = true,
}

-- Ways → roads table
function osm2pgsql.process_way(object)
    local hw = object.tags.highway
    if hw == nil or not highway_keep[hw] then return end

    tables.roads:insert({
        name           = object.tags.name,
        name_en        = object.tags['name:en'],
        highway        = hw,
        cycleway       = object.tags.cycleway,
        cycleway_left  = object.tags['cycleway:left'],
        cycleway_right = object.tags['cycleway:right'],
        bicycle        = object.tags.bicycle,
        surface        = object.tags.surface,
        smoothness     = object.tags.smoothness,
        maxspeed       = object.tags.maxspeed,
        oneway         = object.tags.oneway,
        lit            = object.tags.lit,
        width          = object.tags.width,
        lanes          = object.tags.lanes,
        tags           = object.tags,
        geom           = object:as_linestring(),
    })
end

-- Nodes → pois table
function osm2pgsql.process_node(object)
    local t = object.tags
    local keep = false

    if t.amenity   and poi_amenity[t.amenity]   then keep = true end
    if t.shop      and poi_shop[t.shop]          then keep = true end
    if t.leisure   and poi_leisure[t.leisure]    then keep = true end
    if t.highway   and poi_highway[t.highway]    then keep = true end

    if not keep then return end

    tables.pois:insert({
        name          = t.name,
        name_en       = t['name:en'],
        amenity       = t.amenity,
        shop          = t.shop,
        leisure       = t.leisure,
        highway       = t.highway,
        tourism       = t.tourism,
        brand         = t.brand,
        operator      = t.operator,
        opening_hours = t.opening_hours,
        phone         = t.phone,
        website       = t.website,
        tags          = t,
        geom          = object:as_point(),
    })
end

-- Relations/areas → landuse table
function osm2pgsql.process_relation(object)
    local t = object.tags
    if t.type ~= 'multipolygon' then return end

    local keep = false
    if t.landuse and (t.landuse == 'forest' or t.landuse == 'grass'
                      or t.landuse == 'meadow' or t.landuse == 'farmland') then
        keep = true
    end
    if t.leisure == 'park' or t.leisure == 'nature_reserve' then keep = true end
    if t.natural then keep = true end

    if not keep then return end

    tables.landuse:insert({
        name    = t.name,
        landuse = t.landuse,
        leisure = t.leisure,
        natural_ = t.natural,
        tags    = t,
        geom    = object:as_multipolygon(),
    })
end
