import type { ManagementStatus } from "@/services/alertManagement";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { MANAGEMENT_STATUS_STYLES } from "../../config/alertManagementDisplay";

export function ManagementStatusBadge({ status }: { status: ManagementStatus }) {
    const { label, badgeClass, Icon } = MANAGEMENT_STATUS_STYLES[status];
    return (
        <Badge className={cn("gap-1", badgeClass)}>
            <Icon aria-hidden />
            {label}
        </Badge>
    );
}
