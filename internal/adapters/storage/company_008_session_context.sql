-- Stable optional context stays inside immutable row_json. This additive index
-- supports counts by service/VLAN on the same observation, without a new ledger.
CREATE INDEX pppoe_rows_service_vlan ON pppoe_rows(
 observation_id, json_extract(row_json,'$.context.service'), json_extract(row_json,'$.context.vlan')
);
