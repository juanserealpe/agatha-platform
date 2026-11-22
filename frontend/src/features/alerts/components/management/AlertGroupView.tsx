import { useId, useMemo, useState } from "react";
import { toAlertId, type AlertDetail } from "@/services/alertManagement";
import { DialogTitle } from "@/components/ui/dialog";
import { cn } from "@/lib/utils";
import {
    useAlertGroup,
    usePrefetchGroupDetails,
    useSelectGroupTab,
    useTabNovelty,
    useWaitForGroup,
} from "../../hooks/useAlertGroup";
import { useAlertManagementDetail } from "../../hooks/useAlertManagementDetail";
import { useTimelineNavigation, type TimelineNavigation } from "../../hooks/useTimelineNavigation";
import { memberCurrentLevel, type GroupCandidate } from "../../lib/alertGroup";
import { DEFAULT_MANAGEMENT_MODE, type ManagementMode } from "../../lib/managementMode";
import { compactStripHeightPx, type StripIndicatorInput } from "../../lib/timeStrip";
import { AlertContentSkeleton, AlertDetailSkeleton, TimeStripSkeleton } from "./AlertDetailSkeleton";
import { AlertTabs, TAB_LIST_HEIGHT_PX } from "./AlertTabs";
import { DetailLoadState } from "./DetailLoadState";
import { ManagementContent } from "./ManagementContent";
import { ManagementHeader } from "./ManagementHeader";
import { TimeStrip } from "./TimeStrip";

// Aire entre la franja compacta y el título de la fase a la que se navega, para que se lea dónde se está.
const PHASE_SCROLL_GAP_PX = 12;
// Separación entre la franja y las pestañas dentro del bloque fijo.
const TABS_GAP_PX = 12;
// Relleno del modal (p-5 en AlertManagementDialog).
const MODAL_PADDING_PX = 20;
// Zona del punto: fondo gris de lado a lado del modal (cubre su relleno p-5).
const POINT_ZONE_CLASS = "-mx-5 bg-muted px-5";

interface AlertGroupViewProps {
    /** Alerta con que se abrió el modal: da el punto y el modo de gestión inicial. */
    anchor: AlertDetail;
    /** Pestaña abierta (la de la URL). */
    activeId: number;
    initialMode: ManagementMode;
    onClose: () => void;
}

/**
 * Vista combinada (spec multiindicador §3, A2): cabecera con los valores generales del grupo, franja de tiempo y,
 * con varias alertas, una pestaña por alerta con su detalle y su gestión. Con una sola alerta se ve como antes:
 * sin pestañas.
 *
 * Las pestañas visitadas quedan montadas (ocultas) mientras el modal esté abierto, así cada una conserva lo que el
 * usuario dejó: fases desplegadas, paso de gestión y formulario.
 */
export function AlertGroupView({ anchor, activeId, initialMode, onClose }: AlertGroupViewProps) {
    const idPrefix = useId();
    const deviceId = anchor.device.id;
    const activeDetail = useAlertManagementDetail(toAlertId(activeId)).data;
    const group = useAlertGroup(activeDetail ?? anchor, deviceId);
    const { members } = group;
    const combined = members.length > 1;
    const { selectTab, prefetchTab } = useSelectGroupTab(members, deviceId);
    const navigation = useTimelineNavigation(activeId, selectTab);
    const novelty = useTabNovelty(group, activeId);
    usePrefetchGroupDetails(members, activeId);
    const waitingForGroup = useWaitForGroup(group.loading);

    // Pestañas visitadas, en el orden en que se abrieron (patrón de React para derivar estado de props).
    const [visited, setVisited] = useState<readonly number[]>([activeId]);
    if (!visited.includes(activeId)) setVisited([...visited, activeId]);

    const stripIndicators = useMemo<StripIndicatorInput[]>(
        () =>
            members.map((member) => ({
                id: member.id,
                event: member.event,
                levels: member.levels,
                endedAt: member.state === "ACTIVA" ? null : member.ended_at,
                code: combined ? member.code : undefined,
                current: member.id === activeId,
            })),
        [members, combined, activeId]
    );

    const activeMember = members.find((member) => member.id === activeId) ?? activeDetail ?? anchor;
    const header = headerAlert(activeMember, anchor);
    const tabsHeightPx = combined ? TAB_LIST_HEIGHT_PX + TABS_GAP_PX : 0;
    const scrollMarginTopPx = compactStripHeightPx(stripIndicators.length) + tabsHeightPx + PHASE_SCROLL_GAP_PX;
    const tabId = (alertId: number) => `${idPrefix}-tab-${alertId}`;
    const panelId = (alertId: number) => `${idPrefix}-panel-${alertId}`;

    if (waitingForGroup) {
        return (
            <>
                <DialogTitle className="sr-only">Gestión de alerta {anchor.code}</DialogTitle>
                <AlertDetailSkeleton />
            </>
        );
    }

    return (
        <>
            {/* Con varias alertas el modal tiene dos zonas: la del PUNTO (cabecera, franja y pestañas) sobre fondo gris
                y la de la ALERTA de la pestaña abierta sobre blanco; la pestaña abierta, blanca, une las dos. */}
            <div className={cn(combined && POINT_ZONE_CLASS, combined && "-mt-5 pt-5 pb-5")}>
                <ManagementHeader alert={header} group={members} />
            </div>
            {/* -mx-5/px-5 y stickyTopPx: al quedar fija se pega al borde del modal y cubre su relleno (p-5).
                Las pestañas van dentro del mismo bloque fijo (§5.3: "fija junto con las pestañas"). Con varias alertas,
                -mt-5 anula el espacio de la grilla para que el fondo gris de la zona del punto sea continuo. */}
            {group.loading ? (
                // El grupo tardó más que la espera: no se dibuja la franja de una sola alerta, porque al llegar el grupo
                // cambiaría a varias barras y pestañas y parecería que se abrió otra alerta.
                <TimeStripSkeleton />
            ) : (
                <TimeStrip
                    indicators={stripIndicators}
                    selected={navigation.selected}
                    onSelectPhase={navigation.navigateToPhase}
                    onSelectAlert={selectTab}
                    onPrefetchAlert={prefetchTab}
                    hoverRange={navigation.hoverRange}
                    className={cn("-mx-5 px-5", combined && cn(POINT_ZONE_CLASS, "-mt-5"))}
                    stickyTopPx={-MODAL_PADDING_PX}
                    below={
                        combined && (
                            <AlertTabs
                                members={members}
                                activeId={activeId}
                                novelty={novelty}
                                onSelect={selectTab}
                                onPrefetch={prefetchTab}
                                tabId={tabId}
                                panelId={panelId}
                            />
                        )
                    }
                    belowHeightPx={tabsHeightPx}
                />
            )}
            {visited.map((alertId) => (
                <div
                    key={alertId}
                    id={panelId(alertId)}
                    role={combined ? "tabpanel" : undefined}
                    aria-labelledby={combined ? tabId(alertId) : undefined}
                    hidden={alertId !== activeId}
                    className="flex flex-col gap-5"
                >
                    <AlertTabPanel
                        alertId={alertId}
                        initialMode={alertId === anchor.id ? initialMode : DEFAULT_MANAGEMENT_MODE}
                        onClose={onClose}
                        navigation={navigation.forAlert(alertId)}
                        scrollMarginTopPx={scrollMarginTopPx}
                        combined={combined}
                    />
                </div>
            ))}
        </>
    );
}

/** La cabecera muestra la alerta de la pestaña abierta; mientras su detalle carga, con el resumen del punto. */
function headerAlert(member: GroupCandidate, anchor: AlertDetail) {
    return {
        code: member.code,
        event: member.event,
        device: anchor.device,
        in_progress: member.state === "ACTIVA",
        current_level: memberCurrentLevel(member),
    };
}

interface AlertTabPanelProps {
    alertId: number;
    initialMode: ManagementMode;
    onClose: () => void;
    navigation: TimelineNavigation;
    scrollMarginTopPx: number;
    combined: boolean;
}

/** Contenido de una pestaña: el detalle de SU alerta (spec §6), con su propia carga y su propio error. */
function AlertTabPanel({ alertId, initialMode, onClose, navigation, scrollMarginTopPx, combined }: AlertTabPanelProps) {
    const query = useAlertManagementDetail(toAlertId(alertId));
    if (!query.data) return <DetailLoadState query={query} skeleton={<AlertContentSkeleton />} />;
    return (
        <ManagementContent
            detail={query.data}
            initialMode={initialMode}
            onClose={onClose}
            navigation={navigation}
            scrollMarginTopPx={scrollMarginTopPx}
            combined={combined}
        />
    );
}
