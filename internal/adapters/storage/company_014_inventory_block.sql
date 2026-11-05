-- Add local grouping without rewriting existing device or operation records.
ALTER TABLE devices ADD COLUMN block TEXT NOT NULL DEFAULT '' CHECK(length(block)<=80);
CREATE TABLE device_inventory_roles (
 device_id TEXT PRIMARY KEY REFERENCES devices(id),
 role TEXT NOT NULL CHECK(role IN ('edge','switch','station','power-monitor','ups','rectifier','solar-controller','battery-monitor'))
);
