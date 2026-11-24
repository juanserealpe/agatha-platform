// Package models son las filas de las tablas de Alert Management (escritura)
// y los modelos de solo lectura de rule_evaluations/rule_threshold_versions
// (Rule Engine) y devices (Ingestion), que no se migran.
package models

import "time"

// AlertPolicyVersionModel es una versión de la política (solo se agrega).
// measuring_point_id NULL = global. El EXCLUDE de vigencias lo agrega
// persistence.Migrate con SQL (btree_gist).
type AlertPolicyVersionModel struct {
	ID               int64  `gorm:"column:id;primaryKey;autoIncrement"`
	Version          int    `gorm:"column:version;type:integer;not null"`
	MeasuringPointID *int64 `gorm:"column:measuring_point_id;index:ix_alert_policy_versions_effective,priority:1"`
	EventType        string `gorm:"column:event_type;type:text;not null;default:'FUGA';index:ix_alert_policy_versions_effective,priority:2"`

	DowngradeHours                        int     `gorm:"column:downgrade_hours;type:integer;not null;check:chk_alert_policy_downgrade_hours,downgrade_hours >= 1"`
	RojoRequiresClassificationToDowngrade bool    `gorm:"column:rojo_requires_classification_to_downgrade;not null"`
	MergeWindowHours                      int     `gorm:"column:merge_window_hours;type:integer;not null;check:chk_alert_policy_merge_window_hours,merge_window_hours >= 1"`
	ReactivateOnlyIfUnclassified          bool    `gorm:"column:reactivate_only_if_unclassified;not null"`
	InterventionSilentMaxLevel            string  `gorm:"column:intervention_silent_max_level;type:text;not null;check:chk_alert_policy_intervention_level,intervention_silent_max_level IN ('AMARILLO', 'NARANJA')"`
	SummaryEveryHours                     int     `gorm:"column:summary_every_hours;type:integer;not null;check:chk_alert_policy_summary_hours,summary_every_hours >= 1"`
	CountdownHours                        int     `gorm:"column:countdown_hours;type:integer;not null;check:chk_alert_policy_countdown_hours,countdown_hours >= 1"`
	TrendLookbackHours                    int     `gorm:"column:trend_lookback_hours;type:integer;not null;check:chk_alert_policy_trend_lookback_hours,trend_lookback_hours >= 1"`
	TrendDelta                            float64 `gorm:"column:trend_delta;type:numeric(5,4);not null;check:chk_alert_policy_trend_delta,trend_delta > 0 AND trend_delta < 1"`
	OscillationFlips                      int     `gorm:"column:oscillation_flips;type:integer;not null;check:chk_alert_policy_oscillation_flips,oscillation_flips >= 1"`
	OscillationWindowHours                int     `gorm:"column:oscillation_window_hours;type:integer;not null;check:chk_alert_policy_oscillation_window,oscillation_window_hours >= 1"`
	LateNotificationMaxHours              int     `gorm:"column:late_notification_max_hours;type:integer;not null;check:chk_alert_policy_late_hours,late_notification_max_hours >= 0"`

	ValidFrom time.Time  `gorm:"column:valid_from;type:timestamptz;not null;index:ix_alert_policy_versions_effective,priority:3,sort:desc"`
	ValidTo   *time.Time `gorm:"column:valid_to;type:timestamptz;check:chk_alert_policy_validity,valid_to IS NULL OR valid_to > valid_from"`
	Reason    string     `gorm:"column:reason;type:text;not null"`
	CreatedBy string     `gorm:"column:created_by;type:text;not null"`
	CreatedAt time.Time  `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
}

func (AlertPolicyVersionModel) TableName() string { return "alert_policy_versions" }

// AlertModel es una alerta. code es GENERATED (lo agrega Migrate con SQL: los
// tags no lo expresan y AutoMigrate intentaría alterarlo). Los CHECK de
// gestión y el trigger de inmutabilidad también los agrega Migrate. Una ACTIVA
// por (punto, evento): índice único parcial ux_alerts_one_active.
type AlertModel struct {
	ID               int64  `gorm:"column:id;primaryKey;autoIncrement"`
	Code             string `gorm:"column:code;->;-:migration"`
	MeasuringPointID int64  `gorm:"column:measuring_point_id;not null;index:ix_alerts_point_started,priority:1"`
	EventType        string `gorm:"column:event_type;type:text;not null;default:'FUGA';check:chk_alerts_event_type,event_type IN ('FUGA', 'MOVIMIENTO')"`
	State            string `gorm:"column:state;type:text;not null;check:chk_alerts_state,state IN ('ACTIVA', 'FINALIZADA')"`

	// Contexto del movimiento (§7.1). Alerta de MOVIMIENTO: copiado de su
	// última hora calculada. Alerta de FUGA: solo la temperatura, si le llegó un TEMP_*.
	MotionState           *string    `gorm:"column:motion_state;type:text"`
	TempState             *string    `gorm:"column:temp_state;type:text"`
	TempStableFrom        *time.Time `gorm:"column:temp_stable_from;type:timestamptz"`
	TempStableTo          *time.Time `gorm:"column:temp_stable_to;type:timestamptz"`
	LastMotionEvaluatedAt *time.Time `gorm:"column:last_motion_evaluated_at;type:timestamptz"`
	// motion-v2 (§9.1, aditivas): el historial del OPENED que abrió o
	// reactivó la alerta y el nivel calculado máximo.
	MotionHistory    *string `gorm:"column:motion_history;type:jsonb"`
	MotionTableLevel *string `gorm:"column:motion_table_level;type:text;check:chk_alerts_motion_table_level,motion_table_level IS NULL OR motion_table_level IN ('AMARILLO', 'NARANJA', 'ROJO')"`

	StartedAt         time.Time  `gorm:"column:started_at;type:timestamptz;not null;index:ix_alerts_point_started,priority:2,sort:desc;index:ix_alerts_started"`
	OpenedAt          time.Time  `gorm:"column:opened_at;type:timestamptz;not null"`
	EndedAt           *time.Time `gorm:"column:ended_at;type:timestamptz;check:chk_alerts_ended,(state = 'ACTIVA') = (ended_at IS NULL)"`
	ReactivationCount int        `gorm:"column:reactivation_count;type:integer;not null;default:0"`
	LastReactivatedAt *time.Time `gorm:"column:last_reactivated_at;type:timestamptz"`

	CurrentLevel string `gorm:"column:current_level;type:text;not null;check:chk_alerts_current_level,current_level IN ('AMARILLO', 'NARANJA', 'ROJO')"`
	MaxLevel     string `gorm:"column:max_level;type:text;not null;check:chk_alerts_max_level,max_level IN ('AMARILLO', 'NARANJA', 'ROJO')"`

	OpeningIndicatorValue float64 `gorm:"column:opening_indicator_value;type:numeric;not null"`
	LastIndicatorValue    float64 `gorm:"column:last_indicator_value;type:numeric;not null"`
	// MaxIndicatorValue: default 0 para las filas previas; Migrate las completa.
	MaxIndicatorValue float64   `gorm:"column:max_indicator_value;type:numeric;not null;default:0"`
	LastEvaluatedAt   time.Time `gorm:"column:last_evaluated_at;type:timestamptz;not null"`

	HoursCalculated int `gorm:"column:hours_calculated;type:integer;not null;default:0"`
	HoursNoData     int `gorm:"column:hours_no_data;type:integer;not null;default:0"`
	BelowLevelHours int `gorm:"column:below_level_hours;type:integer;not null;default:0"`

	LowConfidenceAtOpen bool   `gorm:"column:low_confidence_at_open;not null"`
	Runtime             string `gorm:"column:runtime;type:jsonb;not null;default:'{}'"`

	PolicyVersionID          int64 `gorm:"column:policy_version_id;not null"`
	ThresholdVersionIDAtOpen int64 `gorm:"column:threshold_version_id_at_open;not null"`
	OpenedByRuleEvaluationID int64 `gorm:"column:opened_by_rule_evaluation_id;not null"`

	ManagementStatus         string     `gorm:"column:management_status;type:text;not null;default:'SIN_CLASIFICAR';index:ix_alerts_management_status"`
	Classification           *string    `gorm:"column:classification;type:text;check:chk_alerts_classification,classification IS NULL OR classification IN ('CONFIRMADA', 'FALSA_ALERTA')"`
	ClassifiedAt             *time.Time `gorm:"column:classified_at;type:timestamptz"`
	ClassifiedOrigin         *string    `gorm:"column:classified_origin;type:text;check:chk_alerts_classified_origin,classified_origin IS NULL OR classified_origin IN ('WEB', 'APP_MOVIL')"`
	ClassifiedDevice         *string    `gorm:"column:classified_device;type:text"`
	ClassifiedRecordedAt     *time.Time `gorm:"column:classified_recorded_at;type:timestamptz"`
	ClassifiedClientRecordID *string    `gorm:"column:classified_client_record_id;type:uuid"`

	Label                 *string    `gorm:"column:label;type:text"`
	LabelOtherText        *string    `gorm:"column:label_other_text;type:text;check:chk_alerts_label_other_text_length,label_other_text IS NULL OR char_length(label_other_text) BETWEEN 1 AND 100"`
	LabeledAt             *time.Time `gorm:"column:labeled_at;type:timestamptz"`
	LabeledOrigin         *string    `gorm:"column:labeled_origin;type:text;check:chk_alerts_labeled_origin,labeled_origin IS NULL OR labeled_origin IN ('WEB', 'APP_MOVIL')"`
	LabeledDevice         *string    `gorm:"column:labeled_device;type:text"`
	LabeledRecordedAt     *time.Time `gorm:"column:labeled_recorded_at;type:timestamptz"`
	LabeledClientRecordID *string    `gorm:"column:labeled_client_record_id;type:uuid"`

	Source    string    `gorm:"column:source;type:text;not null;check:chk_alerts_source,source IN ('LIVE', 'CATCH_UP', 'BACKFILL')"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
}

func (AlertModel) TableName() string { return "alerts" }

// AlertLevelSegmentModel es un color de la alerta.
type AlertLevelSegmentModel struct {
	ID        int64      `gorm:"column:id;primaryKey;autoIncrement"`
	AlertID   int64      `gorm:"column:alert_id;not null;uniqueIndex:ux_alert_level_segments_alert_seq,priority:1"`
	Alert     AlertModel `gorm:"foreignKey:AlertID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Seq       int        `gorm:"column:seq;type:integer;not null;uniqueIndex:ux_alert_level_segments_alert_seq,priority:2"`
	OpenedBy  string     `gorm:"column:opened_by;type:text;not null;check:chk_alert_level_segments_opened_by,opened_by IN ('OPEN', 'UP', 'DOWN', 'REACTIVATION')"`
	Level     string     `gorm:"column:level;type:text;not null;check:chk_alert_level_segments_level,level IN ('AMARILLO', 'NARANJA', 'ROJO')"`
	StartedAt time.Time  `gorm:"column:started_at;type:timestamptz;not null"`
	EndedAt   *time.Time `gorm:"column:ended_at;type:timestamptz"`
}

func (AlertLevelSegmentModel) TableName() string { return "alert_level_segments" }

// AlertMessageModel es un mensaje (solo se agrega).
type AlertMessageModel struct {
	ID              int64                  `gorm:"column:id;primaryKey;autoIncrement"`
	AlertID         int64                  `gorm:"column:alert_id;not null;uniqueIndex:ux_alert_messages_alert_hour_code,priority:1"`
	Alert           AlertModel             `gorm:"foreignKey:AlertID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	SegmentID       int64                  `gorm:"column:segment_id;not null"`
	Segment         AlertLevelSegmentModel `gorm:"foreignKey:SegmentID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	EvaluatedAt     time.Time              `gorm:"column:evaluated_at;type:timestamptz;not null;uniqueIndex:ux_alert_messages_alert_hour_code,priority:2"`
	Kind            string                 `gorm:"column:kind;type:text;not null;index:ix_alert_messages_kind;check:chk_alert_messages_kind,kind IN ('NOTIFICACION', 'ACTUALIZACION', 'RESUMEN', 'FIN', 'GESTION')"`
	Code            string                 `gorm:"column:code;type:text;not null;uniqueIndex:ux_alert_messages_alert_hour_code,priority:3"`
	Level           string                 `gorm:"column:level;type:text;not null"`
	IndicatorValue  *float64               `gorm:"column:indicator_value;type:numeric"`
	Title           string                 `gorm:"column:title;type:text;not null"`
	Body            string                 `gorm:"column:body;type:text;not null"`
	Params          string                 `gorm:"column:params;type:jsonb;not null;default:'{}'"`
	CatalogVersion  int                    `gorm:"column:catalog_version;type:integer;not null"`
	PolicyVersionID int64                  `gorm:"column:policy_version_id;not null"`
	CreatedAt       time.Time              `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
}

func (AlertMessageModel) TableName() string { return "alert_messages" }

// AlertObservationModel es una observación (solo se agrega).
type AlertObservationModel struct {
	ID               int64      `gorm:"column:id;primaryKey;autoIncrement"`
	AlertID          int64      `gorm:"column:alert_id;not null;index:ix_alert_observations_alert"`
	Alert            AlertModel `gorm:"foreignKey:AlertID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Text             string     `gorm:"column:text;type:text;not null;check:chk_alert_observations_text,char_length(text) BETWEEN 1 AND 500"`
	Kind             string     `gorm:"column:kind;type:text;not null;check:chk_alert_observations_kind,kind IN ('OBSERVACION', 'CONFLICTO_CLASIFICACION', 'CONFLICTO_ETIQUETA')"`
	Step             string     `gorm:"column:step;type:text;not null;check:chk_alert_observations_step,step IN ('CLASIFICACION', 'ETIQUETA', 'LIBRE')"`
	Origin           string     `gorm:"column:origin;type:text;not null;check:chk_alert_observations_origin,origin IN ('WEB', 'APP_MOVIL') AND (origin = 'WEB' OR device_identifier IS NOT NULL)"`
	DeviceIdentifier *string    `gorm:"column:device_identifier;type:text"`
	ClientRecordID   *string    `gorm:"column:client_record_id;type:uuid;uniqueIndex:ux_alert_observations_client_record"`
	RecordedAt       time.Time  `gorm:"column:recorded_at;type:timestamptz;not null"`
	ReceivedAt       time.Time  `gorm:"column:received_at;type:timestamptz;not null;default:now()"`
}

func (AlertObservationModel) TableName() string { return "alert_observations" }

// AlertProcessingStateModel es el estado del procesamiento por (punto, evento).
type AlertProcessingStateModel struct {
	MeasuringPointID    int64      `gorm:"column:measuring_point_id;primaryKey;autoIncrement:false"`
	EventType           string     `gorm:"column:event_type;type:text;primaryKey"`
	LastEvaluatedAt     *time.Time `gorm:"column:last_evaluated_at;type:timestamptz"`
	PendingRunStartedAt *time.Time `gorm:"column:pending_run_started_at;type:timestamptz"`
	LastAlertID         *int64     `gorm:"column:last_alert_id"`
	LastStatus          string     `gorm:"column:last_status;type:text;not null;default:''"`
	UpdatedAt           time.Time  `gorm:"column:updated_at;type:timestamptz;not null;default:now()"`
}

func (AlertProcessingStateModel) TableName() string { return "alert_processing_state" }

// RuleEvaluationReadModel es una fila de rule_evaluations con los parámetros
// de su versión de umbrales (JOIN). Solo lectura: las tablas son del Rule Engine.
type RuleEvaluationReadModel struct {
	ID                 int64     `gorm:"column:id"`
	MeasuringPointID   int64     `gorm:"column:measuring_point_id"`
	EventType          string    `gorm:"column:event_type"`
	EvaluatedAt        time.Time `gorm:"column:evaluated_at"`
	IndicatorValue     float64   `gorm:"column:indicator_value"`
	RawLevel           string    `gorm:"column:raw_level"`
	ComputedLevel      string    `gorm:"column:computed_level"`
	Level              string    `gorm:"column:level"`
	InEpisode          bool      `gorm:"column:in_episode"`
	HoursAbove         int       `gorm:"column:hours_above"`
	RunAmarillo        int       `gorm:"column:run_amarillo"`
	RunNaranja         int       `gorm:"column:run_naranja"`
	BelowRun           int       `gorm:"column:below_run"`
	ThresholdVersionID int64     `gorm:"column:threshold_version_id"`
	LowConfidence      bool      `gorm:"column:low_confidence"`
	EpisodeEvent       string    `gorm:"column:episode_event"`
	Reasons            string    `gorm:"column:reasons"`

	Amarillo                   float64 `gorm:"column:amarillo"`
	Naranja                    float64 `gorm:"column:naranja"`
	Rojo                       float64 `gorm:"column:rojo"`
	ExitMargin                 float64 `gorm:"column:exit_margin"`
	EscalateAmarilloAfterHours *int    `gorm:"column:escalate_amarillo_after_hours"`
	EscalateNaranjaAfterHours  *int    `gorm:"column:escalate_naranja_after_hours"`
	PersistenceMode            string  `gorm:"column:persistence_mode"`
	PersistenceMinHours        *int    `gorm:"column:persistence_min_hours"`
}

// ReasonReadRow es un elemento de rule_evaluations.reasons.
type ReasonReadRow struct {
	Code  string `json:"code"`
	Level string `json:"level"`
	Data  struct {
		Hours int    `json:"hours"`
		From  string `json:"from"`
	} `json:"data"`
}

// DeviceReadModel es una fila de devices (de Ingestion).
type DeviceReadModel struct {
	MeasuringPointID int64  `gorm:"column:measuring_point_id"`
	Name             string `gorm:"column:name"`
}

// MotionEvaluationReadModel es una fila de motion_evaluations con el
// alerts_enabled de su versión (JOIN). Solo lectura: las tablas son del Rule Engine.
type MotionEvaluationReadModel struct {
	ID               int64      `gorm:"column:id"`
	MeasuringPointID int64      `gorm:"column:measuring_point_id"`
	EvaluatedAt      time.Time  `gorm:"column:evaluated_at"`
	Status           string     `gorm:"column:status"`
	MotionState      *string    `gorm:"column:motion_state"`
	TempState        *string    `gorm:"column:temp_state"`
	TempMode         string     `gorm:"column:temp_mode"`
	ComputedLevel    string     `gorm:"column:computed_level"`
	InEpisode        bool       `gorm:"column:in_episode"`
	EpisodeEvent     string     `gorm:"column:episode_event"`
	Displaced        bool       `gorm:"column:displaced"`
	PitchNow         *float64   `gorm:"column:pitch_now"`
	RollNow          *float64   `gorm:"column:roll_now"`
	RefPitch         *float64   `gorm:"column:ref_pitch"`
	RefRoll          *float64   `gorm:"column:ref_roll"`
	ShiftStartedAt   *time.Time `gorm:"column:shift_started_at"`
	GapFrom          *time.Time `gorm:"column:gap_from"`
	GapTo            *time.Time `gorm:"column:gap_to"`
	TempStableFrom   *time.Time `gorm:"column:temp_stable_from"`
	TempStableTo     *time.Time `gorm:"column:temp_stable_to"`
	Reasons          string     `gorm:"column:reasons"`
	// motion-v2 (NULL en las filas motion-v1).
	TableLevel         *string `gorm:"column:table_level"`
	EpisodeKind        *string `gorm:"column:episode_kind"`
	History            *string `gorm:"column:history"`
	RulesHit           *string `gorm:"column:rules_hit"`
	ThresholdVersionID int64   `gorm:"column:threshold_version_id"`
	AlertsEnabled      bool    `gorm:"column:alerts_enabled"`
}
