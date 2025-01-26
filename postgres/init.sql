-- Create tenant table (just for the example)
CREATE TABLE tenants
(
    id   UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL
);



