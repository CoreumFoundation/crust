/* ---- PARAMS ---- */

CREATE TABLE customparams_params
(
    one_row_id     BOOLEAN NOT NULL DEFAULT TRUE PRIMARY KEY,
    staking_params JSONB   NOT NULL,
    height         BIGINT  NOT NULL,
    CHECK (one_row_id)
);

CREATE INDEX customparams_params_height_index ON customparams_params (height);
