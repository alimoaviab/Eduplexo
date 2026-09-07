import { ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import { AppIcon } from "shared/ui/AppIcon";
import { Button, Select, Skeleton } from "@/components/ui";
import { useCampusGuard, Campus } from "@/hooks/useCampusGuard";
import { useRolePath } from "@/hooks/useRolePath";
import { useAuth } from "@/hooks/useAuth";

interface Props {
  children: ReactNode;
  entityName?: string; // e.g. "academic session", "class", "teacher"
  selectedCampusId?: string;
  onCampusChange?: (campusId: string) => void;
  showCampusSelect?: boolean;
}

export function CampusRequirementGuard({
  children,
  entityName = "record",
  selectedCampusId = "",
  onCampusChange,
  showCampusSelect = true,
}: Props) {
  const navigate = useNavigate();
  const { rolePath } = useRolePath();
  const { user } = useAuth();
  const { isLoading, hasSchools, campuses, activeCampusId, selectBranch } = useCampusGuard();

  if (isLoading) {
    return (
      <div className="p-6 space-y-4">
        <Skeleton className="h-12 w-full rounded-xl" />
        <Skeleton className="h-48 w-full rounded-xl" />
      </div>
    );
  }

  const effectiveCampusId = selectedCampusId || activeCampusId;

  return (
    <div className="space-y-4">
      {showCampusSelect && campuses.length > 1 && (
        <div className="rounded-2xl border border-slate-200 bg-slate-50/80 p-4">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div className="flex items-center gap-2">
              <AppIcon name="GitBranch" size={18} className="text-emerald-600" />
              <div>
                <label className="text-[11px] font-bold text-slate-800 block">
                  Select Campus / Branch *
                </label>
                <span className="text-[10px] font-medium text-slate-500">
                  Select which campus this {entityName} belongs to
                </span>
              </div>
            </div>
            <div className="w-full sm:w-64">
              <Select
                value={effectiveCampusId}
                onChange={(e) => {
                  const val = e.target.value;
                  selectBranch(val);
                  if (onCampusChange) onCampusChange(val);
                }}
                options={[
                  { label: "Select Campus...", value: "" },
                  ...campuses.map((c: Campus) => ({
                    label: c.name,
                    value: c._id || c.id || "",
                  })),
                ]}
              />
            </div>
          </div>
        </div>
      )}
      {children}
    </div>
  );
}
