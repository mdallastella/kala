-- Namespace names are stored normalised (lowercase, no leading '#' or
-- surrounding spaces), like the tags they match. A name whose normalised
-- form already exists for the user is left as it is.
UPDATE OR IGNORE namespaces SET name = lower(trim(ltrim(trim(name), '#')))
WHERE name <> lower(trim(ltrim(trim(name), '#')));
