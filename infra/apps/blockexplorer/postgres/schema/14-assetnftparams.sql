/* ---- PARAMS ---- */

CREATE TABLE assetnft_params
(
    one_row_id BOOLEAN NOT NULL DEFAULT TRUE PRIMARY KEY,
    params     JSONB   NOT NULL,
    height     BIGINT  NOT NULL,
    CHECK (one_row_id)
);
CREATE INDEX assetnft_params_height_index ON assetnft_params (height);
