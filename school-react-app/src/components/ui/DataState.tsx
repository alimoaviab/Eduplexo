import { AppIcon } from "shared/ui/AppIcon";
import { Card } from "./Card";

type DataStateVariant = "loading" | "empty" | "error" | "success" | "info";

const variantTokens: Record<
  DataStateVariant,
  { icon: string; iconColor: string; iconBg: string; titleColor: string }
> = {
  loading: { icon: "progress_activity", iconColor: "text-blue-600", iconBg: "bg-blue-50", titleColor: "text-slate-900" },
  empty: { icon: "inbox", iconColor: "text-slate-500", iconBg: "bg-slate-100", titleColor: "text-slate-900" },
  error: { icon: "error", iconColor: "text-red-600", iconBg: "bg-red-50", titleColor: "text-red-700" },
  success: { icon: "check_circle", iconColor: "text-emerald-600", iconBg: "bg-emerald-50", titleColor: "text-emerald-700" },
  info: { icon: "info", iconColor: "text-blue-600", iconBg: "bg-blue-50", titleColor: "text-slate-900" },
};

export interface DataStateProps {
  variant: DataStateVariant;
  title: string;
  message?: string;
  onRetry?: () => void;
  retryLabel?: string;
  icon?: string;
  inline?: boolean;
  action?: React.ReactNode;
}

export function DataState({
  variant,
  title,
  message,
  onRetry,
  retryLabel = "Try again",
  icon,
  inline = false,
  action,
}: DataStateProps) {
  const tokens = variantTokens[variant];
  const iconName = icon ?? tokens.icon;
  const isSpinning = variant === "loading";

  const inner = (
    <div className="flex flex-col items-center justify-center gap-4 px-4 py-8 text-center md:px-6 md:py-10">
      <div className={`flex h-14 w-14 items-center justify-center rounded-2xl ${tokens.iconBg}`}>
        <AppIcon name={iconName} size={26} className={` text-[26px] ${tokens.iconColor} ${isSpinning ? "animate-spin" : ""}`}
          aria-hidden="true" />
      </div>
      <div className="space-y-1.5">
        <h3 className={`text-base font-bold tracking-tight ${tokens.titleColor}`}>{title}</h3>
        {message && (
          <p className="mx-auto max-w-md text-sm leading-relaxed text-slate-500">{message}</p>
        )}
      </div>
      {variant === "error" && onRetry && (
        <button
          type="button"
          onClick={onRetry}
          className="inline-flex items-center gap-2 rounded-xl bg-red-50 px-4 py-2 text-sm font-semibold text-red-700 transition-colors hover:bg-red-100"
        >
          <AppIcon name="RefreshCw" size={18} />
          {retryLabel}
        </button>
      )}
      {action && (
        <div className="pt-1 flex items-center justify-center gap-2">
          {action}
        </div>
      )}
    </div>
  );

  if (inline) return inner;

  return <Card className="border-dashed">{inner}</Card>;
}
