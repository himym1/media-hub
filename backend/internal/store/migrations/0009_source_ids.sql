UPDATE subscriptions
SET source_ids_json = replace(replace(source_ids_json, '"frame"', '"framehdr"'), '"gather"', '"juying"'),
    preferences_json = replace(replace(preferences_json, '"frame"', '"framehdr"'), '"gather"', '"juying"');
