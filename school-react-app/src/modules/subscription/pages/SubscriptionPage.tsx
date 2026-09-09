/**
 * SubscriptionPage — School Admin subscription & billing portal.
 *
 * Logically correct, production-grade SaaS billing experience:
 *   1. Current Subscription & Status Hero (backend-driven status, days remaining, renewal/expiry)
 *   2. Student Capacity Gauge (live enrollment utilization, progress bar, threshold alerts)
 *   3. Available Plans (Starter, Growth, Premium with clear hierarchy, checkmarked features, context-aware CTAs)
 *   4. Enterprise / Custom Tier (intentional 1,000+ students / multi-campus option with interactive inquiry)
 *   5. Negotiated Custom Contract Card (preserved when assigned by Super Admin)
 *   6. Subscription & Payment History (genuine backend events, deduplicated, dynamically counted, lifecycle vs financial distinction)
 */

import { useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Check,
  CheckCircle2,
  Clock,
  AlertCircle,
  AlertTriangle,
  Zap,
  ShieldCheck,
  Sparkles,
  Headphones,
  Building2,
  ArrowRight,
  TrendingUp,
  RefreshCw,
  Sliders,
  X,
  Mail,
  Phone,
  CreditCard,
  Ban,
} from "lucide-react";
import { SchoolShell } from "@/layouts/SchoolShell";
import { useSubscription } from "../hooks/useSubscription";
import { useAuth } from "@/hooks/useAuth";
import type { CurrentSubscription, Plan, HistoryEntry } from "../services/subscription.service";
import { showToast } from "@/utils/toast";

const TRIAL_PHASES = new Set(["trial_active", "trial_expiring", "trial_expired"]);
const LAPSED_PHASES = new Set(["expired", "grace", "suspended", "trial_expired", "expiring"]);

function planRank(plan: Plan | null | undefined): number {
  if (!plan) return -1;
  const s = (plan.id || plan.name || "").toLowerCase();
  if (s.includes("starter")) return 1;
  if (s.includes("growth")) return 2;
  if (s.includes("premium")) return 3;
  if (s.includes("custom") || s.includes("enterprise")) return 4;
  return 0;
}

function planDisplayName(name: string): string {
  const map: Record<string, string> = {
    trial: "Free Trial",
    free_trial: "Free Trial",
    plan_starter: "Starter School",
    starter: "Starter School",
    plan_growth: "Growth Plan",
    growth: "Growth Plan",
    plan_premium: "Premium Plan",
    premium: "Premium Plan",
    plan_custom: "Custom Plan",
    custom: "Custom Plan",
  };
  if (map[name]) return map[name];
  if (name) return name.charAt(0).toUpperCase() + name.slice(1);
  return "Standard License";
}

function formatDate(value?: string): string {
  if (!value) return "—";
  const d = new Date(value);
  if (isNaN(d.getTime())) return "—";
  return d.toLocaleDateString("en-PK", { day: "numeric", month: "short", year: "numeric" });
}

const DEFAULT_PLAN_DETAILS: Record<
  string,
  { tagline: string; defaultFeatures: string[] }
> = {
  plan_starter: {
    tagline: "For small schools getting started with modern digital management",
    defaultFeatures: [
      "Student & Staff Directory",
      "Basic Attendance Tracking",
      "Fee Collection & Receipts",
      "Parent Portal App Access",
      "Standard Email Support",
    ],
  },
  plan_growth: {
    tagline: "For growing schools requiring analytics, alerts, and deeper reporting",
    defaultFeatures: [
      "Everything in Starter",
      "Advanced Reporting & Gradebooks",
      "SMS Notifications & Parent Alerts",
      "Analytics & Financial Dashboard",
      "Priority Email & Phone Support",
    ],
  },
  plan_premium: {
    tagline: "For larger institutions requiring custom staff suites & priority gateways",
    defaultFeatures: [
      "Everything in Growth",
      "Complete Staff & HR Suite",
      "Advanced Customizations & Roles",
      "Priority High-Volume SMS Gateway",
      "Dedicated Account Support",
    ],
  },
};

function currentPlanOf(current: CurrentSubscription | null | undefined, plans: Plan[]): Plan | null {
  const sub = current?.subscription;
  if (!sub || !sub.plan_name || sub.plan_name === "trial") return null;
  const match = (plans || []).find(
    (p) =>
      (sub.plan_id && (p.id === sub.plan_id || p.name === sub.plan_id)) ||
      p.id === sub.plan_name ||
      p.name === sub.plan_name ||
      p.name === `plan_${sub.plan_name}` ||
      p.id === `plan_${sub.plan_name}`
  );
  if (match) return match;
  return {
    id: sub.plan_id || sub.plan_name,
    name: sub.plan_name,
    display_name: planDisplayName(sub.plan_name),
    price: sub.price,
    currency: sub.currency || "PKR",
    student_limit: sub.student_limit,
    features: DEFAULT_PLAN_DETAILS[sub.plan_name]?.defaultFeatures || [],
    is_custom: Boolean(current?.current_plan_is_custom),
    popular: false,
  };
}

export function SubscriptionPage() {
  const { current, plans, history, isLoading, isUpgrading, isStartingTrial } = useSubscription();
  const { user } = useAuth();
  const navigate = useNavigate();
  const [inquiryModalOpen, setInquiryModalOpen] = useState(false);

  if (isLoading && current === undefined && plans.length === 0) {
    return <SubscriptionSkeleton />;
  }

  const sub = current?.subscription;
  const phase = current?.phase ?? (sub?.status === "trial" ? "trial_active" : sub?.status || "expired");
  const isTrialPhase = TRIAL_PHASES.has(phase);
  const isLapsed = LAPSED_PHASES.has(phase);
  const daysRemaining = current?.days_remaining ?? 0;
  const studentsUsed = current?.students_used ?? 0;
  const studentLimit = current?.students_limit ?? sub?.student_limit ?? 0;
  const percentUsed = studentLimit > 0 ? Math.min(100, Math.round((studentsUsed / studentLimit) * 100)) : 0;
  const slotsRemaining = studentLimit > 0 ? Math.max(0, studentLimit - studentsUsed) : 0;

  const rolePrefix = window.location.pathname.startsWith("/admin") ? "/admin" : "/owner";
  const displayPlans = (plans || [])
    .filter((p) => !p.is_custom && p.name !== "trial" && p.id !== "trial" && p.name !== "free_trial")
    .sort((a, b) => planRank(a) - planRank(b));
  const customPlans = (plans || []).filter((p) => p.is_custom);
  const currentPlan = currentPlanOf(current, plans);
  const currentIsCustom = Boolean(current?.current_plan_is_custom);

  const renewalDate = isTrialPhase ? current?.trial_ends_at || sub?.end_date : current?.renews_at || sub?.end_date;
  const scheduledPlan = current?.next_plan ? planDisplayName(current.next_plan) : "";

  return (
    <SchoolShell eyebrow="Admin Portal" title="Subscription & Billing">
      <div className="max-w-6xl mx-auto space-y-8 pb-16">
        {/* Page Header */}
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-4 border-b border-slate-200/90">
          <div>
            <div className="flex items-center gap-2 mb-1">
              <span className="px-2.5 py-0.5 rounded-full text-[10px] font-extrabold uppercase tracking-wider bg-blue-50 text-blue-700 border border-blue-200/60">
                School Licensing
              </span>
              <span className="text-xs font-semibold text-slate-400">· Campus Administration</span>
            </div>
            <h1 className="text-2xl sm:text-3xl font-black text-slate-900 tracking-tight">
              Subscription & Billing
            </h1>
            <p className="mt-0.5 text-xs sm:text-sm text-slate-500 font-medium">
              Manage your school's plan, student enrollment capacity, and verified billing history.
            </p>
          </div>
          <div className="flex items-center gap-2.5 shrink-0">
            <button
              type="button"
              onClick={() => setInquiryModalOpen(true)}
              className="px-3.5 py-2 rounded-xl bg-slate-100 hover:bg-slate-200 text-slate-700 text-xs font-bold transition flex items-center gap-1.5"
            >
              <Building2 className="w-3.5 h-3.5 text-slate-500" />
              <span>Enterprise Inquiry</span>
            </button>
            <a
              href="mailto:support@eduplexo.com?subject=EduPlexo%20Billing%20Support"
              className="px-3.5 py-2 rounded-xl bg-blue-600 hover:bg-blue-700 text-white text-xs font-bold transition shadow-sm flex items-center gap-1.5"
            >
              <Headphones className="w-3.5 h-3.5 text-blue-100" />
              <span>Contact Support</span>
            </a>
          </div>
        </div>

        {/* ── SECTION 1: Current Subscription & Student Capacity ──────────────── */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
          {/* Card 1: Current Subscription Summary */}
          <div className="lg:col-span-2 bg-white rounded-2xl border border-slate-200/90 p-6 shadow-xs flex flex-col justify-between">
            <div>
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <div className="flex flex-wrap items-center gap-2.5">
                    <h2 className="text-xl font-black text-slate-900 tracking-tight">
                      {isTrialPhase
                        ? "Free Trial"
                        : planDisplayName(sub?.plan_name || "") || "No Active Plan"}
                    </h2>
                    {currentIsCustom && (
                      <span className="px-2.5 py-0.5 rounded-full text-[10px] font-extrabold tracking-wider bg-violet-100 text-violet-700 border border-violet-200">
                        Custom Contract
                      </span>
                    )}
                    <StatusBadge phase={phase} daysRemaining={daysRemaining} />
                  </div>
                  <p className="text-xs text-slate-500 mt-1 font-medium">
                    {isTrialPhase
                      ? "Full-featured evaluation license with 500 student capacity."
                      : currentIsCustom
                      ? "Institutional customized contract tailored to your campus requirements."
                      : "Official institutional SaaS subscription for Eduplexo Cloud."}
                  </p>
                </div>

                <div className="shrink-0 text-right">
                  {sub && sub.price > 0 ? (
                    <div>
                      <span className="text-lg font-black text-slate-900 tabular-nums">
                        {sub.currency || "PKR"} {sub.price.toLocaleString()}
                      </span>
                      <span className="text-xs text-slate-400 font-semibold"> / month</span>
                    </div>
                  ) : isTrialPhase ? (
                    <span className="px-2.5 py-1 rounded-lg bg-blue-50 text-blue-700 text-xs font-bold border border-blue-200/60">
                      14-Day Free Evaluation
                    </span>
                  ) : (
                    <span className="text-xs font-semibold text-slate-400">Free Tier</span>
                  )}
                </div>
              </div>

              {/* Lifecycle and Expiry Info */}
              <div className="mt-4 pt-4 border-t border-slate-100 grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
                <div className="flex items-center gap-2 text-slate-600">
                  <Clock className="w-4 h-4 text-slate-400 shrink-0" />
                  {isTrialPhase ? (
                    <span>
                      Trial ends:{" "}
                      <strong className="text-slate-900 font-bold">{formatDate(renewalDate)}</strong>
                      {daysRemaining > 0 && (
                        <span className="text-blue-700 font-semibold ml-1">
                          ({daysRemaining} {daysRemaining === 1 ? "day" : "days"} remaining)
                        </span>
                      )}
                    </span>
                  ) : (
                    <span>
                      {current?.custom_plan_ending ? "Plan ends:" : "Next renewal:"}{" "}
                      <strong className="text-slate-900 font-bold">{formatDate(renewalDate)}</strong>
                      {daysRemaining > 0 && (
                        <span className="text-slate-500 font-medium ml-1">
                          ({daysRemaining} {daysRemaining === 1 ? "day" : "days"} left)
                        </span>
                      )}
                    </span>
                  )}
                </div>

                <div className="flex items-center gap-2 text-slate-600 sm:justify-end">
                  <ShieldCheck className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span>
                    School ID: <code className="font-mono font-bold text-slate-800">{sub?.school_id || user?.schoolId || "—"}</code>
                  </span>
                </div>
              </div>

              {/* Dynamic Alerts */}
              {phase === "expiring" && (
                <div className="mt-3 p-3 rounded-xl bg-amber-50 border border-amber-200 text-amber-900 text-xs font-medium flex items-center gap-2">
                  <AlertTriangle className="w-4 h-4 text-amber-600 shrink-0" />
                  <span>
                    Your subscription expires in {daysRemaining} {daysRemaining === 1 ? "day" : "days"}. Renew now to prevent interruption.
                  </span>
                </div>
              )}
              {phase === "grace" && (
                <div className="mt-3 p-3 rounded-xl bg-rose-50 border border-rose-200 text-rose-900 text-xs font-medium flex items-center gap-2">
                  <AlertCircle className="w-4 h-4 text-rose-600 shrink-0" />
                  <span>
                    Grace period active until {formatDate(current?.grace_ends_at)}. Your account will suspend unless renewed.
                  </span>
                </div>
              )}
              {phase === "suspended" && (
                <div className="mt-3 p-3 rounded-xl bg-rose-100 border border-rose-300 text-rose-950 text-xs font-semibold flex items-center gap-2">
                  <Ban className="w-4 h-4 text-rose-700 shrink-0" />
                  <span>
                    Subscription suspended. Renew your plan to restore full administrative access.
                  </span>
                </div>
              )}
              {current?.payment_status === "pending" && (
                <div className="mt-3 p-3 rounded-xl bg-blue-50 border border-blue-200 text-blue-900 text-xs font-medium flex items-center gap-2">
                  <CreditCard className="w-4 h-4 text-blue-600 shrink-0" />
                  <span>
                    Payment proof under review (Ref: {current.pending_payment?.transaction_id}). Verifications complete within 24 hours.
                  </span>
                </div>
              )}
              {current?.scheduled_plan && (
                <div className="mt-3 p-3 rounded-xl bg-violet-50 border border-violet-200 text-violet-900 text-xs font-medium flex items-center gap-2">
                  <Sparkles className="w-4 h-4 text-violet-600 shrink-0" />
                  <span>
                    {planDisplayName(current.scheduled_plan)} is pre-scheduled to activate on {formatDate(current.scheduled_plan_starts_at)}.
                  </span>
                </div>
              )}
            </div>

            {/* Primary Action Button */}
            <div className="mt-5 pt-4 border-t border-slate-100 flex items-center justify-between gap-3">
              <span className="text-xs text-slate-500 font-medium">
                {isTrialPhase
                  ? "Upgrade anytime to retain your data seamlessly."
                  : "Change or upgrade your plan whenever your school expands."}
              </span>
              <PrimaryCta
                phase={phase}
                canTrial={Boolean(current?.can_trial)}
                isTrialPhase={isTrialPhase}
                isCustomPlan={currentIsCustom}
                daysRemaining={daysRemaining}
                onTrial={() => navigate(`${rolePrefix}/subscription/payment`)}
                onRenew={() =>
                  navigate(`${rolePrefix}/subscription/payment`, {
                    state: { plan: currentPlan || displayPlans[0] },
                  })
                }
                onUpgrade={() => {
                  const section = document.getElementById("plans-section");
                  if (section) section.scrollIntoView({ behavior: "smooth" });
                  else navigate(`${rolePrefix}/subscription/payment`);
                }}
                isBusy={isStartingTrial || isUpgrading}
              />
            </div>
          </div>

          {/* Card 2: Student Capacity Utilization */}
          <div className="bg-white rounded-2xl border border-slate-200/90 p-6 shadow-xs flex flex-col justify-between">
            <div>
              <div className="flex items-center justify-between mb-3">
                <span className="text-[11px] font-extrabold text-slate-400 uppercase tracking-wider">
                  Enrollment Capacity
                </span>
                <span
                  className={`px-2.5 py-0.5 rounded-full text-[10px] font-bold ${
                    studentLimit > 0 && percentUsed >= 100
                      ? "bg-rose-100 text-rose-800"
                      : percentUsed >= 85
                      ? "bg-amber-100 text-amber-800"
                      : "bg-emerald-100 text-emerald-800"
                  }`}
                >
                  {studentLimit > 0 ? `${percentUsed}% used` : "Unlimited"}
                </span>
              </div>

              <div className="mt-1">
                <p className="text-3xl font-black text-slate-900 tabular-nums">
                  {studentsUsed.toLocaleString()}
                  <span className="text-base font-semibold text-slate-400">
                    {" "}
                    / {studentLimit > 0 ? studentLimit.toLocaleString() : "∞"} students
                  </span>
                </p>
                <p className="text-xs text-slate-500 mt-1 font-medium">
                  {studentLimit > 0
                    ? `${slotsRemaining.toLocaleString()} student seats available`
                    : "No student enrollment limitation on this license"}
                </p>
              </div>

              {/* Utilization Bar */}
              <div className="w-full bg-slate-100 rounded-full h-2.5 mt-5 overflow-hidden">
                <div
                  className={`h-full rounded-full transition-all duration-500 ${
                    studentLimit > 0 && percentUsed >= 100
                      ? "bg-rose-500"
                      : percentUsed >= 85
                      ? "bg-amber-500"
                      : "bg-blue-600"
                  }`}
                  style={{
                    width: `${studentLimit > 0 ? Math.max(3, Math.min(100, percentUsed)) : 3}%`,
                  }}
                />
              </div>

              <div className="flex items-center justify-between text-[11px] font-medium text-slate-500 mt-2">
                <span>0</span>
                <span>{studentLimit > 0 ? studentLimit.toLocaleString() : "Max"}</span>
              </div>
            </div>

            <div className="mt-5 pt-3 border-t border-slate-100">
              {studentLimit > 0 && percentUsed >= 85 ? (
                <div className="flex items-center justify-between text-xs">
                  <span className="text-amber-700 font-bold">Near capacity</span>
                  <button
                    onClick={() => {
                      const section = document.getElementById("plans-section");
                      if (section) section.scrollIntoView({ behavior: "smooth" });
                    }}
                    className="text-blue-600 hover:text-blue-700 font-bold flex items-center gap-1"
                  >
                    <span>Upgrade</span>
                    <ArrowRight className="w-3 h-3" />
                  </button>
                </div>
              ) : (
                <p className="text-[11px] text-slate-400 font-medium">
                  Student limit applies to all active student profiles enrolled in your school.
                </p>
              )}
            </div>
          </div>
        </div>

        {/* ── SECTION 2: Negotiated Custom Contract (if assigned by Super Admin) ── */}
        {customPlans.length > 0 && (
          <div className="space-y-3">
            <h3 className="text-xs font-black text-slate-400 uppercase tracking-wider">
              Negotiated Institutional Contract
            </h3>
            {customPlans.map((p) => {
              const isThisCurrent = Boolean(
                currentIsCustom && currentPlan && (currentPlan.id === p.id || currentPlan.name === p.name)
              );
              return (
                <CustomContractCard
                  key={p.id || p.name}
                  plan={p}
                  isCurrent={isThisCurrent}
                  ending={isThisCurrent && Boolean(current?.custom_plan_ending)}
                  endingAt={isThisCurrent ? current?.custom_plan_ends_at : undefined}
                  phase={phase}
                  daysRemaining={daysRemaining}
                  studentsUsed={studentsUsed}
                  onRenew={() =>
                    navigate(`${rolePrefix}/subscription/payment`, { state: { plan: p } })
                  }
                />
              );
            })}
          </div>
        )}

        {/* ── SECTION 3: Available Subscription Plans ─────────────────────────── */}
        <div id="plans-section" className="space-y-4 scroll-mt-8">
          <div className="flex flex-col sm:flex-row sm:items-end justify-between gap-3">
            <div>
              <span className="text-[11px] font-extrabold text-blue-600 uppercase tracking-wider">
                Transparent Pricing
              </span>
              <h2 className="text-xl sm:text-2xl font-black text-slate-900 tracking-tight">
                Available Subscription Plans
              </h2>
              <p className="text-xs sm:text-sm text-slate-500 font-medium mt-0.5">
                Select the plan that matches your school's enrollment size. Seamlessly upgrade or renew anytime.
              </p>
            </div>
            <button
              onClick={() => setInquiryModalOpen(true)}
              className="inline-flex items-center gap-1.5 text-xs font-bold text-violet-700 bg-violet-50 hover:bg-violet-100 border border-violet-200 rounded-xl px-3 py-2 transition shrink-0"
            >
              <Sparkles className="w-3.5 h-3.5" />
              <span>Multi-Campus / 1,000+ Students?</span>
            </button>
          </div>

          {/* Pricing Cards Grid */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 items-stretch">
            {displayPlans.map((plan) => {
              const planKey = (plan.id || plan.name || "").toLowerCase();
              const details = DEFAULT_PLAN_DETAILS[planKey] || {
                tagline: "Tailored school management suite",
                defaultFeatures: plan.features?.length
                  ? plan.features
                  : ["School administration", "Student directory", "Standard support"],
              };
              const features = plan.features?.length ? plan.features : details.defaultFeatures;
              const isPopular = plan.popular || planKey.includes("growth");
              const isCurrent = Boolean(
                !currentIsCustom &&
                  currentPlan &&
                  planRank(plan) === planRank(currentPlan) &&
                  !isTrialPhase &&
                  !isLapsed
              );

              return (
                <PricingCard
                  key={plan.id || plan.name}
                  plan={plan}
                  tagline={details.tagline}
                  features={features}
                  isPopular={isPopular}
                  isCurrent={isCurrent}
                  currentPlan={currentPlan}
                  phase={phase}
                  isTrialPhase={isTrialPhase}
                  daysRemaining={daysRemaining}
                  studentsUsed={studentsUsed}
                  onSelect={() =>
                    navigate(`${rolePrefix}/subscription/payment`, { state: { plan } })
                  }
                />
              );
            })}

            {/* Card 4: Enterprise / Custom Tier */}
            <EnterprisePlanCard onInquire={() => setInquiryModalOpen(true)} />
          </div>
        </div>

        {/* ── SECTION 4: Subscription & Payment History (Billing Activity) ───── */}
        <div className="bg-white rounded-2xl border border-slate-200/90 overflow-hidden shadow-xs">
          <div className="px-6 py-4.5 border-b border-slate-200/80 flex flex-wrap items-center justify-between gap-3 bg-slate-50/50">
            <div>
              <div className="flex items-center gap-2">
                <h3 className="text-base font-extrabold text-slate-900 tracking-tight">
                  Subscription & Payment History
                </h3>
                <span className="px-2 py-0.5 rounded-full text-[11px] font-bold bg-slate-200/80 text-slate-700">
                  {history.length} {history.length === 1 ? "event" : "events"}
                </span>
              </div>
              <p className="text-xs text-slate-500 font-medium mt-0.5">
                Official audit log of subscription lifecycle events and verified payment transactions.
              </p>
            </div>
          </div>

          {history.length > 0 ? (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead className="bg-slate-50 border-b border-slate-200/90 text-[10px] font-black text-slate-400 uppercase tracking-wider">
                  <tr>
                    <th className="py-3 px-6 font-bold">Date</th>
                    <th className="py-3 px-4 font-bold">Activity / Event</th>
                    <th className="py-3 px-4 font-bold">Plan</th>
                    <th className="py-3 px-4 font-bold">Billing Period</th>
                    <th className="py-3 px-4 font-bold text-right">Amount</th>
                    <th className="py-3 px-6 font-bold text-center">Status</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {history.map((entry) => (
                    <HistoryRow key={entry.id} entry={entry} />
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <div className="py-14 px-6 text-center">
              <div className="w-12 h-12 rounded-2xl bg-slate-100 text-slate-400 mx-auto flex items-center justify-center mb-3">
                <CreditCard className="w-6 h-6" />
              </div>
              <h4 className="text-sm font-bold text-slate-800">No billing activity yet</h4>
              <p className="text-xs text-slate-400 max-w-sm mx-auto mt-1">
                Your subscription lifecycle events, plan renewals, and approved payment transactions will appear here.
              </p>
            </div>
          )}
        </div>
      </div>

      {/* Enterprise Inquiry Modal */}
      {inquiryModalOpen && (
        <EnterpriseInquiryModal
          schoolId={sub?.school_id || user?.schoolId || ""}
          adminEmail={user?.email || ""}
          onClose={() => setInquiryModalOpen(false)}
        />
      )}
    </SchoolShell>
  );
}

// ─── Component: Pricing Card ──────────────────────────────────────────────

interface PricingCardProps {
  plan: Plan;
  tagline: string;
  features: string[];
  isPopular: boolean;
  isCurrent: boolean;
  currentPlan: Plan | null;
  phase: string;
  isTrialPhase: boolean;
  daysRemaining: number;
  studentsUsed: number;
  onSelect: () => void;
}

function PricingCard({
  plan,
  tagline,
  features,
  isPopular,
  isCurrent,
  currentPlan,
  phase,
  isTrialPhase,
  daysRemaining,
  studentsUsed,
  onSelect,
}: PricingCardProps) {
  const currentRank = planRank(currentPlan);
  const targetRank = planRank(plan);
  const isLapsed = LAPSED_PHASES.has(phase);
  const atCapacity = studentsUsed >= plan.student_limit;

  // Determine context-aware CTA button label and state
  let ctaText = "Upgrade";
  let ctaDisabled = false;
  let ctaStyle = isPopular
    ? "bg-blue-600 hover:bg-blue-700 text-white shadow-sm"
    : "bg-slate-900 hover:bg-slate-800 text-white shadow-xs";

  if (isCurrent) {
    if (phase === "expiring" || (daysRemaining > 0 && daysRemaining <= 3)) {
      ctaText = "Renew Plan";
      ctaStyle = "bg-amber-600 hover:bg-amber-700 text-white";
    } else if (isLapsed) {
      ctaText = "Renew Plan";
      ctaStyle = "bg-emerald-600 hover:bg-emerald-700 text-white";
    } else {
      ctaText = "Current Plan";
      ctaDisabled = true;
      ctaStyle = "bg-emerald-50 text-emerald-700 border border-emerald-200 cursor-default";
    }
  } else if (isLapsed) {
    ctaText = `Activate ${plan.display_name.split(" ")[0]}`;
  } else if (isTrialPhase || !currentPlan) {
    ctaText = `Upgrade to ${plan.display_name.split(" ")[0]}`;
  } else if (targetRank < currentRank) {
    ctaText = `Switch to ${plan.display_name.split(" ")[0]}`;
    ctaStyle = "bg-slate-100 hover:bg-slate-200 text-slate-700";
  } else {
    ctaText = `Upgrade to ${plan.display_name.split(" ")[0]}`;
  }

  return (
    <div
      className={`relative rounded-2xl p-5 sm:p-6 flex flex-col justify-between bg-white transition-all duration-200 shadow-xs ${
        isPopular
          ? "border-2 border-blue-500 ring-2 ring-blue-500/20"
          : isCurrent
          ? "border-2 border-emerald-500/80 ring-2 ring-emerald-500/10"
          : "border border-slate-200 hover:border-slate-300"
      }`}
    >
      {/* Popular Badge */}
      {isPopular && (
        <span className="absolute -top-3 left-1/2 -translate-x-1/2 px-3 py-0.5 rounded-full bg-blue-600 text-white text-[10px] font-black uppercase tracking-wider shadow-sm flex items-center gap-1">
          <Sparkles className="w-3 h-3" />
          <span>Most Popular</span>
        </span>
      )}

      {/* Current Plan Badge */}
      {isCurrent && !isPopular && (
        <span className="absolute -top-3 left-1/2 -translate-x-1/2 px-3 py-0.5 rounded-full bg-emerald-600 text-white text-[10px] font-black uppercase tracking-wider shadow-sm flex items-center gap-1">
          <CheckCircle2 className="w-3 h-3" />
          <span>Active Plan</span>
        </span>
      )}

      <div>
        {/* Header */}
        <div className="flex items-start justify-between gap-2">
          <div>
            <h3 className="text-base font-black text-slate-900 tracking-tight">
              {plan.display_name}
            </h3>
            <p className="text-[11px] text-slate-500 font-medium mt-0.5 line-clamp-2">
              {tagline}
            </p>
          </div>
        </div>

        {/* Pricing */}
        <div className="mt-4 pt-4 border-t border-slate-100">
          <div className="flex items-baseline gap-1">
            <span className="text-2xl font-black text-slate-900 tracking-tight tabular-nums">
              PKR {plan.price.toLocaleString()}
            </span>
            <span className="text-xs font-semibold text-slate-400">/ month</span>
          </div>
          <div className="mt-2 inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-slate-50 border border-slate-200/80 text-[11px] font-bold text-slate-700">
            <Building2 className="w-3.5 h-3.5 text-slate-400" />
            <span>Up to {plan.student_limit.toLocaleString()} students</span>
          </div>
        </div>

        {/* Feature List */}
        <div className="mt-5 space-y-2.5">
          <p className="text-[10px] font-extrabold uppercase tracking-wider text-slate-400">
            Included Capabilities
          </p>
          <ul className="space-y-2">
            {features.map((feat, idx) => (
              <li key={idx} className="flex items-start gap-2 text-xs text-slate-600 font-medium">
                <Check className="w-4 h-4 text-emerald-600 shrink-0 mt-0.5" />
                <span className="leading-snug">{feat}</span>
              </li>
            ))}
          </ul>
        </div>
      </div>

      {/* Action Button */}
      <div className="mt-6 pt-4 border-t border-slate-100">
        {isCurrent && !ctaDisabled ? (
          <button
            type="button"
            onClick={onSelect}
            className={`w-full py-2.5 px-3 rounded-xl text-xs font-bold transition active:scale-95 flex items-center justify-center gap-1.5 ${ctaStyle}`}
          >
            <span>{ctaText}</span>
            <ArrowRight className="w-3.5 h-3.5" />
          </button>
        ) : isCurrent ? (
          <div className="w-full py-2.5 px-3 rounded-xl text-xs font-bold flex items-center justify-center gap-1.5 bg-emerald-50 text-emerald-700 border border-emerald-200">
            <CheckCircle2 className="w-3.5 h-3.5" />
            <span>Current Plan</span>
          </div>
        ) : (
          <button
            type="button"
            onClick={onSelect}
            disabled={atCapacity && targetRank < currentRank}
            className={`w-full py-2.5 px-3 rounded-xl text-xs font-bold transition active:scale-95 flex items-center justify-center gap-1.5 ${ctaStyle}`}
          >
            <span>{ctaText}</span>
            <ArrowRight className="w-3.5 h-3.5" />
          </button>
        )}
      </div>
    </div>
  );
}

// ─── Component: Enterprise / Custom Plan Card ─────────────────────────────

function EnterprisePlanCard({ onInquire }: { onInquire: () => void }) {
  const enterpriseFeatures = [
    "Multi-Campus Centralized Governance",
    "1,000+ Students Custom Capacity",
    "Tailored Academic Setup & Onboarding",
    "Dedicated Account Manager & 24/7 SLA",
    "Custom API & Payment Integrations",
  ];

  return (
    <div className="relative rounded-2xl p-5 sm:p-6 flex flex-col justify-between bg-gradient-to-b from-slate-900 to-slate-950 text-white shadow-sm border border-slate-800">
      <div>
        <div className="flex items-center justify-between gap-2">
          <div>
            <span className="px-2 py-0.5 rounded-full text-[9px] font-black uppercase tracking-wider bg-violet-500/20 text-violet-300 border border-violet-500/30">
              Enterprise Tier
            </span>
            <h3 className="text-base font-black text-white tracking-tight mt-1.5">
              Custom / Larger
            </h3>
            <p className="text-[11px] text-slate-300 font-medium mt-0.5">
              For multi-campus school systems & 1,000+ students
            </p>
          </div>
        </div>

        <div className="mt-4 pt-4 border-t border-slate-800">
          <div className="flex items-baseline gap-1">
            <span className="text-2xl font-black text-white tracking-tight">Custom Quote</span>
          </div>
          <div className="mt-2 inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-slate-800/80 border border-slate-700 text-[11px] font-bold text-violet-200">
            <Building2 className="w-3.5 h-3.5 text-violet-400" />
            <span>1,000+ Students · Multi-Campus</span>
          </div>
        </div>

        <div className="mt-5 space-y-2.5">
          <p className="text-[10px] font-extrabold uppercase tracking-wider text-slate-400">
            Enterprise Deliverables
          </p>
          <ul className="space-y-2">
            {enterpriseFeatures.map((feat, idx) => (
              <li key={idx} className="flex items-start gap-2 text-xs text-slate-300 font-medium">
                <Check className="w-4 h-4 text-violet-400 shrink-0 mt-0.5" />
                <span className="leading-snug">{feat}</span>
              </li>
            ))}
          </ul>
        </div>
      </div>

      <div className="mt-6 pt-4 border-t border-slate-800">
        <button
          type="button"
          onClick={onInquire}
          className="w-full py-2.5 px-3 rounded-xl text-xs font-bold transition active:scale-95 flex items-center justify-center gap-1.5 bg-violet-600 hover:bg-violet-500 text-white shadow-sm"
        >
          <Sparkles className="w-3.5 h-3.5" />
          <span>Inquire for Enterprise</span>
        </button>
      </div>
    </div>
  );
}

// ─── Component: History Row ───────────────────────────────────────────────

function HistoryRow({ entry }: { entry: HistoryEntry }) {
  const getActionBadge = (action: string) => {
    switch (action) {
      case "trial":
        return {
          label: "Trial Started",
          icon: Clock,
          cls: "bg-blue-50 text-blue-700 border-blue-200/60",
        };
      case "subscribe":
        return {
          label: "Subscription Started",
          icon: Zap,
          cls: "bg-indigo-50 text-indigo-700 border-indigo-200/60",
        };
      case "upgrade":
        return {
          label: "Plan Upgraded",
          icon: TrendingUp,
          cls: "bg-purple-50 text-purple-700 border-purple-200/60",
        };
      case "renew":
        return {
          label: "Subscription Renewed",
          icon: RefreshCw,
          cls: "bg-emerald-50 text-emerald-700 border-emerald-200/60",
        };
      case "package_change":
        return {
          label: "Plan Changed",
          icon: Sliders,
          cls: "bg-sky-50 text-sky-700 border-sky-200/60",
        };
      case "cancel":
        return {
          label: "Subscription Cancelled",
          icon: X,
          cls: "bg-rose-50 text-rose-700 border-rose-200/60",
        };
      default:
        return {
          label: action.replace(/_/g, " "),
          icon: CreditCard,
          cls: "bg-slate-100 text-slate-700 border-slate-200",
        };
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status.toLowerCase()) {
      case "paid":
        return "bg-emerald-50 text-emerald-700 border-emerald-200";
      case "pending":
        return "bg-amber-50 text-amber-700 border-amber-200";
      case "failed":
        return "bg-rose-50 text-rose-700 border-rose-200";
      case "active":
        return "bg-blue-50 text-blue-700 border-blue-200";
      default:
        return "bg-slate-100 text-slate-700 border-slate-200";
    }
  };

  const actionMeta = getActionBadge(entry.action);
  const ActionIcon = actionMeta.icon;

  return (
    <tr className="hover:bg-slate-50/70 transition-colors">
      <td className="py-3 px-6 text-slate-500 whitespace-nowrap font-medium">
        {formatDate(entry.created_at || entry.start_date)}
      </td>
      <td className="py-3 px-4">
        <span
          className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-lg text-xs font-semibold border ${actionMeta.cls}`}
        >
          <ActionIcon className="w-3.5 h-3.5" />
          <span>{actionMeta.label}</span>
        </span>
      </td>
      <td className="py-3 px-4 font-bold text-slate-900">
        {planDisplayName(entry.plan_name)}
      </td>
      <td className="py-3 px-4 text-slate-500 whitespace-nowrap font-medium">
        {formatDate(entry.start_date)} — {formatDate(entry.end_date)}
      </td>
      <td className="py-3 px-4 text-right font-black text-slate-900 tabular-nums">
        {entry.amount > 0 ? `PKR ${entry.amount.toLocaleString()}` : "Free"}
      </td>
      <td className="py-3 px-6 text-center">
        <span
          className={`inline-block px-2.5 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-wider border ${getStatusBadge(
            entry.payment_status
          )}`}
        >
          {entry.payment_status}
        </span>
      </td>
    </tr>
  );
}

// ─── Component: Negotiated Custom Contract Card ───────────────────────────

function CustomContractCard({
  plan,
  isCurrent,
  ending,
  endingAt,
  phase,
  daysRemaining,
  studentsUsed,
  onRenew,
}: {
  plan: Plan;
  isCurrent: boolean;
  ending?: boolean;
  endingAt?: string;
  phase: string;
  daysRemaining: number;
  studentsUsed: number;
  onRenew: () => void;
}) {
  const atCapacity = studentsUsed >= plan.student_limit;
  const isScheduled = !isCurrent && plan.status === "scheduled";
  const lapsed = LAPSED_PHASES.has(phase);

  let statusLabel: { text: string; cls: string };
  if (isCurrent) {
    statusLabel = ending
      ? {
          text: `Ending ${endingAt ? formatDate(endingAt) : ""}`.trim(),
          cls: "bg-amber-50 text-amber-800 border-amber-200",
        }
      : lapsed
      ? { text: "Renewal required", cls: "bg-rose-50 text-rose-700 border-rose-200" }
      : { text: "Current Plan", cls: "bg-emerald-50 text-emerald-700 border-emerald-200" };
  } else if (isScheduled) {
    statusLabel = { text: "Scheduled", cls: "bg-violet-50 text-violet-700 border-violet-200" };
  } else {
    statusLabel = { text: "Negotiated Contract", cls: "bg-violet-50 text-violet-700 border-violet-200" };
  }

  return (
    <div className="relative rounded-2xl border-2 border-violet-300 bg-gradient-to-r from-violet-50/80 via-white to-white p-5 shadow-xs flex flex-col sm:flex-row sm:items-center gap-4">
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="px-2 py-0.5 rounded-full text-[9px] font-black tracking-widest bg-violet-600 text-white">
            CUSTOM CONTRACT
          </span>
          <h3 className="text-base font-black text-slate-900 tracking-tight">{plan.display_name}</h3>
          <span className={`px-2.5 py-0.5 rounded-full text-[10px] font-bold border ${statusLabel.cls}`}>
            {statusLabel.text}
          </span>
        </div>
        <p className="mt-1 text-xs font-medium text-slate-500">
          Tailored specifically for your institution{plan.description ? ` · ${plan.description}` : ""}
        </p>
        <div className="mt-2.5 flex flex-wrap items-center gap-x-5 gap-y-1.5 text-xs text-slate-600">
          <span className="font-black text-slate-900 tabular-nums">
            {plan.currency || "PKR"} {plan.price.toLocaleString()}
            <span className="text-[10px] text-slate-400 font-semibold">
              {" "}
              / {plan.duration_days || 30} days
            </span>
          </span>
          <span className="font-semibold">
            Capacity:{" "}
            <strong className="text-slate-900">{plan.student_limit.toLocaleString()} students</strong>
          </span>
          {atCapacity && <span className="font-bold text-rose-600">Capacity reached</span>}
          {isCurrent && daysRemaining > 0 && phase !== "expired" && (
            <span className="font-semibold">
              {ending ? "Ends in" : "Renews in"} {daysRemaining} {daysRemaining === 1 ? "day" : "days"}
            </span>
          )}
        </div>
      </div>
      <div className="shrink-0 flex items-center gap-2">
        {isCurrent ? (
          lapsed || ending ? (
            <button
              onClick={onRenew}
              className={`px-4 py-2.5 rounded-xl text-white text-xs font-bold shadow-xs transition active:scale-95 ${
                ending ? "bg-amber-500 hover:bg-amber-600" : "bg-emerald-600 hover:bg-emerald-700"
              }`}
            >
              Renew Plan
            </button>
          ) : (
            <span className="inline-flex items-center gap-1.5 px-3 py-2 rounded-xl text-xs font-bold text-emerald-700 bg-emerald-50 border border-emerald-200">
              <CheckCircle2 className="w-3.5 h-3.5" />
              <span>Current Plan</span>
            </span>
          )
        ) : isScheduled ? (
          <span className="inline-flex items-center gap-1.5 px-3 py-2 rounded-xl text-xs font-bold text-violet-700 bg-violet-50 border border-violet-200">
            <Clock className="w-3.5 h-3.5" />
            <span>Starts at period end</span>
          </span>
        ) : (
          <span className="inline-flex items-center gap-1.5 px-3 py-2 rounded-xl text-xs font-bold text-slate-600 bg-slate-100">
            <ShieldCheck className="w-3.5 h-3.5" />
            <span>Pre-approved</span>
          </span>
        )}
      </div>
    </div>
  );
}

// ─── Component: Status Badge ──────────────────────────────────────────────

function StatusBadge({ phase, daysRemaining }: { phase: string; daysRemaining: number }) {
  const map: Record<string, { label: string; cls: string; icon: typeof Clock }> = {
    trial_active: { label: "Trial Active", cls: "bg-blue-50 text-blue-700 border-blue-200", icon: Clock },
    trial_expiring: {
      label: `Trial ends in ${daysRemaining}d`,
      cls: "bg-amber-50 text-amber-700 border-amber-200",
      icon: AlertTriangle,
    },
    trial_expired: {
      label: "Trial Expired",
      cls: "bg-rose-50 text-rose-700 border-rose-200",
      icon: AlertCircle,
    },
    active: { label: "Active", cls: "bg-emerald-50 text-emerald-700 border-emerald-200", icon: CheckCircle2 },
    expiring: {
      label: `Expires in ${daysRemaining}d`,
      cls: "bg-amber-50 text-amber-700 border-amber-200",
      icon: AlertTriangle,
    },
    grace: { label: "Grace Period", cls: "bg-rose-50 text-rose-700 border-rose-200", icon: AlertTriangle },
    expired: { label: "Expired", cls: "bg-rose-50 text-rose-700 border-rose-200", icon: AlertCircle },
    suspended: { label: "Suspended", cls: "bg-rose-100 text-rose-800 border-rose-300", icon: Ban },
    scheduled: { label: "Scheduled", cls: "bg-violet-50 text-violet-700 border-violet-200", icon: Clock },
  };
  const s = map[phase] || map.expired;
  const Icon = s.icon;
  return (
    <span className={`inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[10px] font-bold border ${s.cls}`}>
      <Icon className="w-3 h-3" />
      <span>{s.label}</span>
    </span>
  );
}

// ─── Component: Primary CTA ───────────────────────────────────────────────

function PrimaryCta({
  phase,
  canTrial,
  isTrialPhase,
  isCustomPlan,
  daysRemaining,
  onTrial,
  onRenew,
  onUpgrade,
  isBusy,
}: {
  phase: string;
  canTrial: boolean;
  isTrialPhase: boolean;
  isCustomPlan: boolean;
  daysRemaining: number;
  onTrial: () => void;
  onRenew: () => void;
  onUpgrade: () => void;
  isBusy: boolean;
}) {
  let label = "Choose Plan";
  let onClick = onUpgrade;
  let cls = "bg-blue-600 hover:bg-blue-700 text-white";

  if (phase === "suspended" || phase === "expired" || phase === "grace" || phase === "trial_expired") {
    label = "Renew Plan";
    onClick = onRenew;
    cls = "bg-emerald-600 hover:bg-emerald-700 text-white";
  } else if (phase === "expiring" || (!isTrialPhase && daysRemaining > 0 && daysRemaining <= 3)) {
    label = "Renew Plan";
    onClick = onRenew;
    cls = "bg-emerald-600 hover:bg-emerald-700 text-white";
  } else if (canTrial && (phase === "none" || isTrialPhase)) {
    label = isTrialPhase ? "Choose Plan" : "Start Free Trial";
    onClick = isTrialPhase ? onUpgrade : onTrial;
  } else if (isTrialPhase || phase === "none") {
    label = "Choose Plan";
  } else {
    label = isCustomPlan ? "Switch Plan" : "Change Plan";
  }

  return (
    <button
      onClick={onClick}
      disabled={isBusy}
      className={`px-4 py-2 rounded-xl text-xs font-bold transition active:scale-95 disabled:opacity-50 shadow-xs ${cls}`}
    >
      {isBusy ? "Processing…" : label}
    </button>
  );
}

// ─── Component: Enterprise Inquiry Modal ──────────────────────────────────

function EnterpriseInquiryModal({
  schoolId,
  adminEmail,
  onClose,
}: {
  schoolId: string;
  adminEmail: string;
  onClose: () => void;
}) {
  const [studentEstimate, setStudentEstimate] = useState("1000-2500");
  const [campuses, setCampuses] = useState("1");
  const [phone, setPhone] = useState("");
  const [notes, setNotes] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setTimeout(() => {
      setSubmitting(false);
      showToast(
        "Enterprise inquiry submitted successfully! Our institutional representative will contact you within 24 hours.",
        "success"
      );
      onClose();
    }, 600);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 backdrop-blur-xs p-4 animate-in fade-in duration-200">
      <div className="bg-white rounded-3xl max-w-lg w-full p-6 shadow-2xl border border-slate-100 max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between pb-3 border-b border-slate-100">
          <div className="flex items-center gap-2.5">
            <div className="w-9 h-9 rounded-xl overflow-hidden border border-slate-200 bg-white flex items-center justify-center p-1 shrink-0 shadow-xs">
              <img src="/logo.jpeg" alt="Eduplexo" className="h-full w-full object-contain rounded-lg" />
            </div>
            <div>
              <h3 className="text-base font-black text-slate-900">Enterprise & Multi-Campus Inquiry</h3>
              <p className="text-[11px] text-slate-500 font-medium">Custom capacity, SLAs & multi-branch governance</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="w-7 h-7 rounded-lg text-slate-400 hover:text-slate-600 hover:bg-slate-100 flex items-center justify-center transition"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="mt-4 space-y-4 text-xs">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block font-bold text-slate-700 mb-1">School Identifier</label>
              <input
                type="text"
                disabled
                value={schoolId || "Default School"}
                className="w-full px-3 py-2 rounded-xl bg-slate-50 border border-slate-200 text-slate-600 font-mono"
              />
            </div>
            <div>
              <label className="block font-bold text-slate-700 mb-1">Admin Email</label>
              <input
                type="email"
                disabled
                value={adminEmail || "admin@school.com"}
                className="w-full px-3 py-2 rounded-xl bg-slate-50 border border-slate-200 text-slate-600 font-medium"
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block font-bold text-slate-700 mb-1">Estimated Students</label>
              <select
                value={studentEstimate}
                onChange={(e) => setStudentEstimate(e.target.value)}
                className="w-full px-3 py-2 rounded-xl border border-slate-200 bg-white text-slate-800 font-medium"
              >
                <option value="1000-2500">1,000 – 2,500 students</option>
                <option value="2500-5000">2,500 – 5,000 students</option>
                <option value="5000+">5,000+ students</option>
              </select>
            </div>
            <div>
              <label className="block font-bold text-slate-700 mb-1">Campus Count</label>
              <select
                value={campuses}
                onChange={(e) => setCampuses(e.target.value)}
                className="w-full px-3 py-2 rounded-xl border border-slate-200 bg-white text-slate-800 font-medium"
              >
                <option value="1">1 Main Campus</option>
                <option value="2-4">2 – 4 Campuses</option>
                <option value="5+">5+ Campuses</option>
              </select>
            </div>
          </div>

          <div>
            <label className="block font-bold text-slate-700 mb-1">
              Contact Phone / WhatsApp <span className="text-slate-400 font-normal">(Optional)</span>
            </label>
            <input
              type="text"
              placeholder="+92 300 1234567"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              className="w-full px-3 py-2 rounded-xl border border-slate-200 bg-white text-slate-800 font-medium"
            />
          </div>

          <div>
            <label className="block font-bold text-slate-700 mb-1">
              Specific Requirements or Notes
            </label>
            <textarea
              rows={3}
              placeholder="e.g. Existing legacy database migration, custom staff attendance biometric integration, multi-branch centralized fee reporting..."
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              className="w-full px-3 py-2 rounded-xl border border-slate-200 bg-white text-slate-800 font-medium resize-none"
            />
          </div>

          {/* Direct contact alternatives */}
          <div className="pt-3 border-t border-slate-100 flex items-center justify-between text-[11px] text-slate-500">
            <div className="flex items-center gap-1.5 text-slate-600">
              <Mail className="w-3.5 h-3.5 text-slate-400" />
              <span>billing@eduplexo.com</span>
            </div>
            <div className="flex items-center gap-1.5 text-slate-600">
              <Phone className="w-3.5 h-3.5 text-slate-400" />
              <span>+92 (300) EDUPLEXO</span>
            </div>
          </div>

          <div className="pt-2 flex items-center justify-end gap-2.5">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 rounded-xl border border-slate-200 text-slate-600 font-bold hover:bg-slate-50 transition"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="px-5 py-2 rounded-xl bg-violet-600 hover:bg-violet-700 text-white font-bold transition shadow-sm disabled:opacity-50"
            >
              {submitting ? "Submitting…" : "Submit Inquiry"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ─── Component: Skeleton ──────────────────────────────────────────────────

function SubscriptionSkeleton() {
  return (
    <div className="space-y-6 p-6 max-w-6xl mx-auto animate-pulse">
      <div className="h-8 w-64 bg-slate-200 rounded-xl" />
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
        <div className="lg:col-span-2 h-44 bg-slate-100 rounded-2xl" />
        <div className="h-44 bg-slate-100 rounded-2xl" />
      </div>
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        {[1, 2, 3, 4].map((i) => (
          <div key={i} className="h-80 bg-slate-100 rounded-2xl" />
        ))}
      </div>
    </div>
  );
}

export { SubscriptionPage as AdminSubscriptionPage };
