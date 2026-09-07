import { useNavigate } from "react-router-dom";
import { AppIcon } from "shared/ui/AppIcon";
import { useAuth } from "@/hooks/useAuth";
import type { CurrentSubscription } from "@/modules/subscription/services/subscription.service";

interface SubscriptionRequiredProps {
  current?: CurrentSubscription | null;
}

export function SubscriptionRequired({ current }: SubscriptionRequiredProps) {
  const navigate = useNavigate();
  const { user } = useAuth();

  const sub = current?.subscription;
  const phase = current?.phase ?? sub?.status;
  const isSuspended = phase === "suspended";
  const isExpired = isSuspended || phase === "expired" || phase === "grace" || phase === "trial_expired" || sub?.status === "cancelled";
  const isAdmin = user?.role === "admin" || user?.role === "super_admin";

  const title = isAdmin
    ? isSuspended
      ? "Account Suspended"
      : isExpired
      ? "Your Subscription Has Expired"
      : "Please Choose Your Subscription Plan"
    : isExpired
      ? "Subscription Expired"
      : "Subscription Inactive";

  const description = isAdmin
    ? isSuspended
      ? "Your school subscription is suspended. Renew your plan to restore access."
      : isExpired
      ? "Your school subscription has ended. Please renew or upgrade your plan to restore full access."
      : "You have not activated your Free Trial or Subscription. Please choose a plan to continue managing your school."
    : isSuspended
      ? "This school's subscription is currently inactive. Please contact your school administrator to renew the EduPlexo subscription."
      : "Your school's subscription plan is currently inactive or has expired. Please contact your school administrator to renew or activate the plan.";

  return (
    <div className="flex flex-1 flex-col items-center justify-center min-h-[70vh] bg-slate-50/50 p-6 md:p-12 animate-fade-in-up">
      <div className="w-full max-w-md text-center bg-white p-8 md:p-10 rounded-3xl border border-slate-100 shadow-xl shadow-slate-200/40 relative overflow-hidden">
        {/* Glow effect */}
        <div className="absolute -top-10 -right-10 w-32 h-32 bg-blue-500/10 rounded-full blur-3xl pointer-events-none" />
        <div className="absolute -bottom-10 -left-10 w-32 h-32 bg-indigo-500/10 rounded-full blur-3xl pointer-events-none" />

        <div className={`h-16 w-16 rounded-2xl flex items-center justify-center mx-auto mb-6 shadow-md ${
          isExpired ? "bg-rose-50 text-rose-500" : "bg-blue-50 text-blue-600"
        }`}>
          <AppIcon name={isExpired ? "AlertTriangle" : "Lock"} size={32} />
        </div>

        <h1 className="text-xl md:text-2xl font-black text-slate-900 tracking-tight mb-3">
          {title}
        </h1>
        
        <p className="text-xs md:text-sm text-slate-500 font-medium leading-relaxed mb-8">
          {description}
        </p>

        {isAdmin ? (
          <div className="space-y-3">
            <button
              onClick={() => navigate("/admin/subscription")}
              className="w-full h-11 md:h-12 rounded-2xl bg-blue-600 hover:bg-blue-700 text-white font-extrabold text-xs md:text-sm tracking-wide shadow-lg shadow-blue-600/20 active:scale-95 transition-all flex items-center justify-center gap-2 cursor-pointer"
            >
              <AppIcon name="Zap" size={16} />
              <span>{isSuspended ? "Renew Plan" : "Choose Subscription Plan"}</span>
              <AppIcon name="ChevronRight" size={16} />
            </button>
          </div>
        ) : (
          <div className="space-y-3">
            <div className="p-4 bg-slate-50 border border-slate-100 rounded-2xl">
              <p className="text-xs font-semibold text-slate-600 leading-normal flex items-center gap-2 justify-center">
                <AppIcon name="Lock" size={14} className="text-slate-400 shrink-0" />
                <span>Please contact your <strong>School Administrator</strong> to renew or activate the subscription.</span>
              </p>
            </div>
            <button
              onClick={() => (window.location.href = "mailto:billing@eduplexo.com")}
              className="w-full h-10 rounded-xl bg-slate-100 hover:bg-slate-200 text-slate-700 font-bold text-xs tracking-wide transition-all flex items-center justify-center gap-2 cursor-pointer"
            >
              <AppIcon name="Headphones" size={14} />
              <span>Contact Support</span>
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
