// Package persistence agrupa el esquema de Alert Management: la migración
// (AutoMigrate + SQL idempotente para lo que los tags de GORM no expresan) y
// los seeds (política global inicial y eventos conocidos).
package persistence

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/models"
)

// RebuildSetting es el parámetro de sesión que habilita el DELETE de alertas
// (solo -rebuild, en una transacción con SET LOCAL).
const RebuildSetting = "agatha.alerts_rebuild"

// PolicySeedValidFrom es la vigencia de la versión global inicial: cubre el
// histórico para el backfill (única excepción a la no retroactividad, como el
// seed del Rule Engine).
var PolicySeedValidFrom = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// Migrate crea/actualiza el esquema. Es idempotente: corre en cada arranque.
func Migrate(db *gorm.DB) error {
	// in_episode llega con v7: con la columna nueva, la fila FUGA de un punto
	// con alerta ACTIVA queda en episodio (antes de v7 toda alerta activa era de FUGA).
	addInEpisode := db.Migrator().HasTable(&models.AlertProcessingStateModel{}) &&
		!db.Migrator().HasColumn(&models.AlertProcessingStateModel{}, "in_episode")

	if err := db.AutoMigrate(
		&models.AlertPolicyVersionModel{},
		&models.AlertModel{},
		&models.AlertLevelSegmentModel{},
		&models.AlertMessageModel{},
		&models.AlertObservationModel{},
		&models.AlertProcessingStateModel{},
	); err != nil {
		return fmt.Errorf("automigrate alert_management: %w", err)
	}

	statements := []string{
		// ALR- + id con 6 dígitos (sin truncar ids de 7 o más dígitos).
		`ALTER TABLE alerts ADD COLUMN IF NOT EXISTS code text
			GENERATED ALWAYS AS ('ALR-' || CASE WHEN id < 1000000 THEN lpad(id::text, 6, '0') ELSE id::text END) STORED`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_alerts_code ON alerts (code)`,
		// v7 (§7.1): una sola ACTIVA por punto, de cualquier origen. Si el índice
		// viejo (punto, evento) existe se reemplaza; la migración aborta si algún
		// punto tiene dos activas (hay que resolverlo a mano antes de desplegar).
		`DO $$ BEGIN
	IF EXISTS (SELECT 1 FROM alerts WHERE state = 'ACTIVA' GROUP BY measuring_point_id HAVING count(*) > 1) THEN
		RAISE EXCEPTION 'alerts: hay puntos con más de una alerta ACTIVA; no se puede crear ux_alerts_one_active por punto';
	END IF;
	IF EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'ux_alerts_one_active' AND indexdef LIKE '%event_type%') THEN
		DROP INDEX ux_alerts_one_active;
	END IF;
END $$`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_alerts_one_active ON alerts (measuring_point_id) WHERE state = 'ACTIVA'`,
		// events: las filas anteriores son solo de su origen; después, NOT NULL.
		`UPDATE alerts SET events = ARRAY[event_type] WHERE events IS NULL OR cardinality(events) = 0`,
		`ALTER TABLE alerts ALTER COLUMN events SET NOT NULL`,
		`CREATE INDEX IF NOT EXISTS ix_alerts_unclassified ON alerts (started_at DESC) WHERE management_status = 'SIN_CLASIFICAR'`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_alert_policy_versions_scope_version
			ON alert_policy_versions (COALESCE(measuring_point_id, 0), event_type, version)`,
		`CREATE EXTENSION IF NOT EXISTS btree_gist`,
		// max_indicator_value llegó con el catálogo v4: las alertas previas
		// toman el mayor índice que conocen (apertura, último y sus mensajes).
		// Aproximado (no todas las horas dejan mensaje); -rebuild lo deja exacto.
		`UPDATE alerts a SET max_indicator_value = GREATEST(a.opening_indicator_value, a.last_indicator_value,
			COALESCE((SELECT max(m.indicator_value) FROM alert_messages m WHERE m.alert_id = a.id), 0))
			WHERE a.max_indicator_value = 0`,
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("migrar alert_management (%s): %w", firstLine(stmt), err)
		}
	}

	constraints := map[string]string{
		"chk_alerts_label_requires_classification": `ALTER TABLE alerts ADD CONSTRAINT chk_alerts_label_requires_classification
			CHECK (label IS NULL OR classification IS NOT NULL)`,
		"chk_alerts_label_list": `ALTER TABLE alerts ADD CONSTRAINT chk_alerts_label_list CHECK (label IS NULL
			OR (classification = 'CONFIRMADA'   AND label IN ('FUGA_CONFIRMADA_CAMPO', 'MOVIMIENTO_TIERRA', 'MAQUINARIA_AJENA', 'OTRO'))
			OR (classification = 'FALSA_ALERTA' AND label IN ('CLIMA_EXTREMO', 'TRANSITO_PESADO', 'MANTENIMIENTO_PROGRAMADO', 'OTRO')))`,
		"chk_alerts_label_other": `ALTER TABLE alerts ADD CONSTRAINT chk_alerts_label_other
			CHECK (COALESCE(label = 'OTRO', false) = (label_other_text IS NOT NULL))`,
		"chk_alerts_management_status": `ALTER TABLE alerts ADD CONSTRAINT chk_alerts_management_status
			CHECK (management_status = CASE WHEN classification IS NULL THEN 'SIN_CLASIFICAR'
			                                WHEN label IS NULL THEN 'CLASIFICADA_SIN_ETIQUETA'
			                                ELSE 'CLASIFICADA_CON_ETIQUETA' END)`,
		"chk_alerts_events": `ALTER TABLE alerts ADD CONSTRAINT chk_alerts_events
			CHECK (cardinality(events) >= 1 AND events <@ ARRAY['FUGA', 'MOVIMIENTO']::text[] AND event_type = ANY (events))`,
		"chk_alerts_temp_state": `ALTER TABLE alerts ADD CONSTRAINT chk_alerts_temp_state CHECK (temp_state IS NULL OR temp_state IN
			('TEMP_VARIABLE', 'VERIFICANDO', 'TEMP_ESTABLE', 'RETORNO_TEMP_ESTABLE', 'SIN_ESTABILIZAR'))`,
		"chk_alerts_motion_state": `ALTER TABLE alerts ADD CONSTRAINT chk_alerts_motion_state CHECK (motion_state IS NULL OR motion_state IN
			('NORMAL', 'SALTO', 'DESPLAZADO', 'RETORNO', 'BRUSCO'))`,
		"ex_alert_policy_versions_no_overlap": `ALTER TABLE alert_policy_versions ADD CONSTRAINT ex_alert_policy_versions_no_overlap
			EXCLUDE USING gist (
				(COALESCE(measuring_point_id, 0)) WITH =,
				event_type WITH =,
				tstzrange(valid_from, valid_to) WITH &&
			)`,
	}
	if addInEpisode {
		if err := db.Exec(`UPDATE alert_processing_state s SET in_episode = true
			WHERE s.event_type = 'FUGA' AND EXISTS (SELECT 1 FROM alerts a
				WHERE a.measuring_point_id = s.measuring_point_id AND a.state = 'ACTIVA')`).Error; err != nil {
			return fmt.Errorf("completar alert_processing_state.in_episode: %w", err)
		}
	}

	for name, stmt := range constraints {
		err := db.Exec(`DO $$ BEGIN
	IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = '` + name + `') THEN
		` + stmt + `;
	END IF;
END $$;`).Error
		if err != nil {
			return fmt.Errorf("crear %s: %w", name, err)
		}
	}

	// Lo escrito en la gestión no cambia desde ningún cliente (RN-06) y las
	// alertas no se borran (salvo -rebuild con SET LOCAL agatha.alerts_rebuild = 'on').
	trigger := `
CREATE OR REPLACE FUNCTION alerts_management_immutable() RETURNS trigger AS $$
BEGIN
	IF TG_OP = 'DELETE' THEN
		IF current_setting('` + RebuildSetting + `', true) = 'on' THEN
			RETURN OLD;
		END IF;
		RAISE EXCEPTION 'alerts: DELETE no permitido';
	END IF;
	IF OLD.classification IS NOT NULL AND
	   (NEW.classification, NEW.classified_at, NEW.classified_origin, NEW.classified_device,
	    NEW.classified_recorded_at, NEW.classified_client_record_id)
	     IS DISTINCT FROM
	   (OLD.classification, OLD.classified_at, OLD.classified_origin, OLD.classified_device,
	    OLD.classified_recorded_at, OLD.classified_client_record_id)
	THEN
		RAISE EXCEPTION 'classification is immutable';
	END IF;
	IF OLD.label IS NOT NULL AND
	   (NEW.label, NEW.label_other_text, NEW.labeled_at, NEW.labeled_origin, NEW.labeled_device,
	    NEW.labeled_recorded_at, NEW.labeled_client_record_id)
	     IS DISTINCT FROM
	   (OLD.label, OLD.label_other_text, OLD.labeled_at, OLD.labeled_origin, OLD.labeled_device,
	    OLD.labeled_recorded_at, OLD.labeled_client_record_id)
	THEN
		RAISE EXCEPTION 'label is immutable';
	END IF;
	RETURN NEW;
END $$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS trg_alerts_management_immutable ON alerts;
CREATE TRIGGER trg_alerts_management_immutable BEFORE UPDATE OR DELETE ON alerts
	FOR EACH ROW EXECUTE FUNCTION alerts_management_immutable();`
	if err := db.Exec(trigger).Error; err != nil {
		return fmt.Errorf("crear el trigger de inmutabilidad de alerts: %w", err)
	}
	return nil
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}

// SeedGlobalPolicy crea la versión 1 global (solo si la tabla está vacía).
func SeedGlobalPolicy(db *gorm.DB) (bool, error) {
	var count int64
	if err := db.Model(&models.AlertPolicyVersionModel{}).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	row := mappers.ToPolicyModel(entities.Policy{
		Version: 1, EventType: entities.EventTypeLeak, PolicyParams: entities.DefaultPolicyParams(),
		ValidFrom: PolicySeedValidFrom, Reason: entities.PolicySeedReason, CreatedBy: entities.SystemCreatedBy,
		CreatedAt: time.Now().UTC(),
	})
	return true, db.Create(&row).Error
}
