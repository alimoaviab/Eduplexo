/**
 * Owner Signup & Brevo Email OTP Verification.
 *
 * Implements a two-stage registration:
 * Stage 1: Owner Details Submission (FullName, Phone, Email, Password)
 * Stage 2: Authoritative 5-Minute 6-Digit OTP Email Verification via Brevo
 */

import { useState, useEffect, type ChangeEvent, type FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { motion, AnimatePresence } from "framer-motion";
import { AppIcon } from "shared/ui/AppIcon";
import { OtpInput } from "../../components/auth/OtpInput";

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const PASSWORD_REGEX = /^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)(?=.*[^A-Za-z0-9\s]).{8,}$/;
const STORAGE_KEY = "eduplexo_pending_signup_session";
const REFERRAL_STORAGE_KEY = "eduplexo_referral_token";

interface PendingSession {
  pendingId: string;
  email: string;
  expiresAt: number; // epoch ms
  resendAt: number;  // epoch ms
}

export function SignupPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();

  // Referral tracking state
  const [referralToken, setReferralToken] = useState<string>("");

  useEffect(() => {
    const ref = searchParams.get("ref") || searchParams.get("referral_token");
    if (ref) {
      const clean = ref.trim();
      sessionStorage.setItem(REFERRAL_STORAGE_KEY, clean);
      setReferralToken(clean);
    } else {
      const stored = sessionStorage.getItem(REFERRAL_STORAGE_KEY);
      if (stored) {
        setReferralToken(stored.trim());
      }
    }
  }, [searchParams]);

  // Form state
  const [stage, setStage] = useState<"form" | "verify">("form");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [successMessage, setSuccessMessage] = useState("");
  const [acceptTerms, setAcceptTerms] = useState(false);

  const [formData, setFormData] = useState({
    schoolName: "",
    fullName: "",
    email: "",
    phone: "",
    password: "",
    confirmPassword: "",
  });

  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);

  // OTP Verification state
  const [pendingSession, setPendingSession] = useState<PendingSession | null>(null);
  const [otpValue, setOtpValue] = useState("");
  const [timeRemaining, setTimeRemaining] = useState<number>(300); // seconds
  const [resendCooldown, setResendCooldown] = useState<number>(60); // seconds
  const [isResending, setIsResending] = useState(false);
  const [isVerifying, setIsVerifying] = useState(false);

  // Change Email Modal state
  const [showChangeEmail, setShowChangeEmail] = useState(false);
  const [newEmailInput, setNewEmailInput] = useState("");
  const [isChangingEmail, setIsChangingEmail] = useState(false);

  // Restore existing pending session on mount
  useEffect(() => {
    try {
      const stored = sessionStorage.getItem(STORAGE_KEY);
      if (stored) {
        const parsed: PendingSession = JSON.parse(stored);
        if (parsed.expiresAt > Date.now()) {
          setPendingSession(parsed);
          setStage("verify");
        } else {
          sessionStorage.removeItem(STORAGE_KEY);
        }
      }
    } catch {
      sessionStorage.removeItem(STORAGE_KEY);
    }
  }, []);

  // Synchronize 5-minute countdown and 60-second resend cooldown timers
  useEffect(() => {
    if (stage !== "verify" || !pendingSession) return;

    const interval = setInterval(() => {
      const now = Date.now();
      const secondsLeft = Math.max(0, Math.floor((pendingSession.expiresAt - now) / 1000));
      const resendLeft = Math.max(0, Math.floor((pendingSession.resendAt - now) / 1000));

      setTimeRemaining(secondsLeft);
      setResendCooldown(resendLeft);

      if (secondsLeft === 0) {
        if (!isResending) {
          setError((prev) => {
            if (prev && prev !== "This verification code has expired. Please request a new code.") {
              return prev;
            }
            return "This verification code has expired. Please request a new code.";
          });
        }
      } else {
        // As long as there is time remaining, clear any stale expiration error
        setError((prev) => (prev === "This verification code has expired. Please request a new code." ? "" : prev));
      }
    }, 1000);

    return () => clearInterval(interval);
  }, [stage, pendingSession, isResending]);

  function handleChange(e: ChangeEvent<HTMLInputElement>) {
    const { name, value } = e.target;
    setFormData(prev => ({ ...prev, [name]: value }));
    setError("");
  }

  function validate(): string | null {
    if (!formData.schoolName.trim()) return "School / institution name is required";
    if (!formData.fullName.trim()) return "Administrator name is required";
    if (!formData.phone.trim()) return "Phone number is required";
    if (!formData.email.trim()) return "Email is required";
    if (!EMAIL_REGEX.test(formData.email)) return "Please enter a valid email address";
    if (!formData.password) return "Password is required";
    if (!PASSWORD_REGEX.test(formData.password)) return "Password must be 8+ chars with uppercase, lowercase, number, and special character";
    if (formData.password !== formData.confirmPassword) return "Passwords do not match";
    if (!acceptTerms) return "You must accept the Terms & Conditions to continue";
    return null;
  }

  // ─── Stage 1: Submit Details & Request 6-Digit OTP ────────────────────────
  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const v = validate();
    if (v) {
      setError(v);
      return;
    }

    setLoading(true);
    setError("");
    setSuccessMessage("");

    try {
      const response = await fetch("/api/auth/signup", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ 
          schoolName: formData.schoolName.trim(),
          fullName: formData.fullName.trim(),
          phone: formData.phone.trim(),
          email: formData.email.trim(),
          password: formData.password,
          role: "admin",
          referral_token: referralToken || sessionStorage.getItem(REFERRAL_STORAGE_KEY) || undefined,
        }),
      });

      const result = await response.json().catch(() => null);
      if (!response.ok) {
        throw new Error(result?.error?.message || result?.message || "Signup failed. Please try again.");
      }

      const data = result?.data;

      // ─── Super Admin Bypass: Direct Login when Skip OTP is active ───
      if (data?.token) {
        sessionStorage.removeItem(STORAGE_KEY);
        sessionStorage.removeItem(REFERRAL_STORAGE_KEY);
        setSuccessMessage("Account created successfully! Redirecting to your dashboard...");

        localStorage.removeItem("active_school_id");
        localStorage.removeItem("active_branch_id");
        localStorage.removeItem("academic_year_id");
        localStorage.removeItem("last_school_id");
        localStorage.removeItem("profile_id");
        localStorage.removeItem("class_id");
        localStorage.removeItem("student_id");

        localStorage.setItem("token", data.token);
        if (data.school_id && data.school_id !== "system") {
          localStorage.setItem("active_school_id", data.school_id);
        }
        window.dispatchEvent(new Event("auth-changed"));
        setTimeout(() => {
          navigate("/admin/dashboard", { replace: true });
        }, 800);
        return;
      }

      if (!data?.pending_id) {
        throw new Error("Invalid response received from authentication server.");
      }

      const expirySeconds = data.expires_in_seconds || 300;
      const cooldownSeconds = data.resend_cooldown_seconds || 60;
      const session: PendingSession = {
        pendingId: data.pending_id,
        email: data.email || formData.email.trim(),
        expiresAt: Date.now() + expirySeconds * 1000,
        resendAt: Date.now() + cooldownSeconds * 1000,
      };

      sessionStorage.setItem(STORAGE_KEY, JSON.stringify(session));
      setPendingSession(session);
      setTimeRemaining(expirySeconds);
      setResendCooldown(cooldownSeconds);
      setOtpValue("");
      setError("");
      setStage("verify");
      setSuccessMessage("We sent a 6-digit verification code to your email address.");
    } catch (err: unknown) {
      setError((err as Error).message || "Signup failed. Please try again.");
    } finally {
      setLoading(false);
    }
  }

  // ─── Stage 2: Verify 6-Digit OTP & Activate Account ───────────────────────
  async function handleVerifyOTP(codeToVerify?: string) {
    const code = codeToVerify || otpValue;
    if (!code || code.length !== 6) {
      setError("Please enter the complete 6-digit verification code.");
      return;
    }
    if (!pendingSession?.pendingId) {
      setError("Verification session not found. Please sign up again.");
      return;
    }

    setIsVerifying(true);
    setError("");
    setSuccessMessage("");

    try {
      const response = await fetch("/api/auth/verify-otp", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          pending_id: pendingSession.pendingId,
          otp: code,
        }),
      });

      const result = await response.json().catch(() => null);
      if (!response.ok) {
        throw new Error(result?.error?.message || result?.message || "Verification failed.");
      }

      // Success! Clear session storage and store auth token
      sessionStorage.removeItem(STORAGE_KEY);
      sessionStorage.removeItem(REFERRAL_STORAGE_KEY);
      setSuccessMessage("Email verified successfully! Redirecting to your dashboard...");

      if (result?.data?.token) {
        // Clear old session storage to prevent cross-account bleed
        localStorage.removeItem("active_school_id");
        localStorage.removeItem("active_branch_id");
        localStorage.removeItem("academic_year_id");
        localStorage.removeItem("last_school_id");
        localStorage.removeItem("profile_id");
        localStorage.removeItem("class_id");
        localStorage.removeItem("student_id");

        localStorage.setItem("token", result.data.token);
        if (result.data.school_id && result.data.school_id !== "system") {
          localStorage.setItem("active_school_id", result.data.school_id);
        }
        window.dispatchEvent(new Event("auth-changed"));
        setTimeout(() => {
          navigate("/admin/dashboard", { replace: true });
        }, 800);
      } else {
        setTimeout(() => navigate("/auth/login"), 800);
      }
    } catch (err: unknown) {
      setError((err as Error).message || "Incorrect verification code. Please try again.");
      setOtpValue("");
    } finally {
      setIsVerifying(false);
    }
  }

  // ─── Stage 2: Resend Fresh 6-Digit OTP ────────────────────────────────────
  async function handleResendOTP() {
    if (resendCooldown > 0 || !pendingSession?.pendingId || isResending) return;

    setIsResending(true);
    setError("");
    setSuccessMessage("");
    // Optimistically reset time remaining so expiration error disappears instantly
    setTimeRemaining(300);

    try {
      const response = await fetch("/api/auth/resend-otp", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          pending_id: pendingSession.pendingId,
        }),
      });

      const result = await response.json().catch(() => null);
      if (!response.ok) {
        throw new Error(result?.error?.message || result?.message || "Failed to resend verification code.");
      }

      const data = result?.data;
      const expirySeconds = data?.expires_in_seconds || 300;
      const cooldownSeconds = data?.resend_cooldown_seconds || 60;

      const updatedSession: PendingSession = {
        ...pendingSession,
        expiresAt: Date.now() + expirySeconds * 1000,
        resendAt: Date.now() + cooldownSeconds * 1000,
      };

      sessionStorage.setItem(STORAGE_KEY, JSON.stringify(updatedSession));
      setPendingSession(updatedSession);
      setTimeRemaining(expirySeconds);
      setResendCooldown(cooldownSeconds);
      setOtpValue("");
      setError("");
      setSuccessMessage("A fresh verification code has been dispatched to your email.");
    } catch (err: unknown) {
      setError((err as Error).message || "Failed to resend code.");
    } finally {
      setIsResending(false);
    }
  }

  // ─── Stage 2: Change Email Address ────────────────────────────────────────
  async function handleChangeEmailSubmit(e: FormEvent) {
    e.preventDefault();
    const cleanEmail = newEmailInput.trim().toLowerCase();
    if (!cleanEmail || !EMAIL_REGEX.test(cleanEmail)) {
      setError("Please enter a valid email address.");
      return;
    }
    if (!pendingSession?.pendingId) return;

    setIsChangingEmail(true);
    setError("");

    try {
      const response = await fetch("/api/auth/change-email", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          pending_id: pendingSession.pendingId,
          new_email: cleanEmail,
        }),
      });

      const result = await response.json().catch(() => null);
      if (!response.ok) {
        throw new Error(result?.error?.message || result?.message || "Failed to update email.");
      }

      const data = result?.data;
      const expirySeconds = data?.expires_in_seconds || 300;
      const cooldownSeconds = data?.resend_cooldown_seconds || 60;

      const updatedSession: PendingSession = {
        ...pendingSession,
        email: cleanEmail,
        expiresAt: Date.now() + expirySeconds * 1000,
        resendAt: Date.now() + cooldownSeconds * 1000,
      };

      sessionStorage.setItem(STORAGE_KEY, JSON.stringify(updatedSession));
      setPendingSession(updatedSession);
      setTimeRemaining(expirySeconds);
      setResendCooldown(cooldownSeconds);
      setOtpValue("");
      setError("");
      setShowChangeEmail(false);
      setSuccessMessage(`Email updated! We sent a new verification code to ${cleanEmail}.`);
    } catch (err: unknown) {
      setError((err as Error).message || "Failed to update email address.");
    } finally {
      setIsChangingEmail(false);
    }
  }

  // Formatted countdown strings
  const formatTimer = (seconds: number) => {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${String(mins).padStart(2, "0")}:${String(secs).padStart(2, "0")}`;
  };

  return (
    <div className="min-h-screen bg-[#f8fafc] relative flex items-center justify-center p-4 md:p-8 overflow-hidden">
      {/* Background with subtle overlay */}
      <div 
        className="absolute inset-0 z-0 bg-cover bg-center bg-no-repeat"
        style={{ backgroundImage: 'url("/school-bg.png")' }}
      />
      <div className="absolute inset-0 z-0 bg-white/40 backdrop-blur-[2px]" />

      <div className="w-full max-w-[500px] relative z-10">
        <motion.div
          layout
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3 }}
          className="bg-white/85 backdrop-blur-2xl rounded-[36px] shadow-[0_30px_90px_rgba(0,0,0,0.12)] border border-white/60 p-8 md:p-10 overflow-hidden"
        >
          {/* Header */}
          <div className="text-center mb-6">
            <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center overflow-hidden rounded-2xl bg-white shadow-md ring-1 ring-black/5">
              <img src="/logo.jpeg" alt="Eduplexo" className="h-full w-full object-cover" />
            </div>

            {stage === "form" ? (
              <>
                {referralToken && (
                  <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-emerald-50 border border-emerald-200 text-emerald-700 text-xs font-semibold mb-3 shadow-sm">
                    <AppIcon name="CheckCircle" size={13} className="text-emerald-600" />
                    <span>Partner Referral Applied: <strong className="font-mono">{referralToken}</strong></span>
                  </div>
                )}
                <h2 className="text-3xl font-black text-gray-900 mb-1 tracking-tight">Create School Account</h2>
                <p className="text-gray-500 font-medium text-xs">Enter your school and administrator details below</p>
              </>
            ) : (
              <>
                <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-blue-50 border border-blue-100 text-blue-700 text-xs font-bold mb-2">
                  <AppIcon name="ShieldCheck" size={14} />
                  <span>Security Verification</span>
                </div>
                <h2 className="text-3xl font-black text-gray-900 mb-1 tracking-tight">Verify Your Email</h2>
                <p className="text-gray-500 font-medium text-xs max-w-xs mx-auto">
                  We've sent a 6-digit verification code to{" "}
                  <strong className="text-gray-800 font-semibold">{pendingSession?.email}</strong>
                </p>
              </>
            )}
          </div>

          <AnimatePresence mode="wait">
            {stage === "form" ? (
              /* ─── Stage 1: Registration Form ────────────────────────────── */
              <motion.form
                key="signup-form"
                initial={{ opacity: 0, x: -20 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: 20 }}
                transition={{ duration: 0.2 }}
                className="space-y-4"
                onSubmit={handleSubmit}
              >
                <Field 
                  label="School / Institution Name" 
                  name="schoolName" 
                  required 
                  value={formData.schoolName} 
                  onChange={handleChange} 
                  placeholder="e.g. Beacon Heights Academy" 
                  autoFocus 
                />

                <Field 
                  label="Administrator Name" 
                  name="fullName" 
                  required 
                  value={formData.fullName} 
                  onChange={handleChange} 
                  placeholder="e.g. Aisha Khan" 
                />
                
                <Field 
                  label="Phone Number" 
                  name="phone" 
                  type="tel" 
                  required 
                  value={formData.phone} 
                  onChange={handleChange} 
                  placeholder="+92 300 1234567" 
                />

                <Field 
                  label="Official Email Address" 
                  name="email" 
                  type="email" 
                  required 
                  value={formData.email} 
                  onChange={handleChange} 
                  placeholder="admin@school.edu" 
                />

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <PasswordField 
                    label="Password" 
                    name="password" 
                    value={formData.password} 
                    onChange={handleChange} 
                    show={showPassword} 
                    onToggle={() => setShowPassword(!showPassword)} 
                  />
                  <PasswordField 
                    label="Confirm Password" 
                    name="confirmPassword" 
                    value={formData.confirmPassword} 
                    onChange={handleChange} 
                    show={showConfirmPassword} 
                    onToggle={() => setShowConfirmPassword(!showConfirmPassword)} 
                  />
                </div>

                <div className="flex items-start gap-3 pt-1">
                  <input
                    id="acceptTerms"
                    name="acceptTerms"
                    type="checkbox"
                    required
                    checked={acceptTerms}
                    onChange={(e) => {
                      setAcceptTerms(e.target.checked);
                      setError("");
                    }}
                    className="w-5 h-5 rounded border-gray-300 text-blue-600 focus:ring-blue-500 accent-blue-600 cursor-pointer mt-0.5"
                  />
                  <label htmlFor="acceptTerms" className="text-xs text-gray-600 font-medium select-none cursor-pointer">
                    I accept the{" "}
                    <a
                      href="https://eduplexo.com/terms"
                      target="_blank"
                      rel="noopener noreferrer"
                      onClick={(e) => e.stopPropagation()}
                      className="text-blue-600 hover:underline font-semibold"
                    >
                      Terms & Conditions
                    </a>{" "}
                    and{" "}
                    <a
                      href="https://eduplexo.com/privacy"
                      target="_blank"
                      rel="noopener noreferrer"
                      onClick={(e) => e.stopPropagation()}
                      className="text-blue-600 hover:underline font-semibold"
                    >
                      Privacy Policy
                    </a>
                    .
                  </label>
                </div>

                {error && (
                  <p className="text-[11px] text-red-600 font-bold bg-red-50/90 p-3.5 rounded-2xl border border-red-200 flex items-center gap-2 shadow-sm">
                    <AppIcon name="AlertCircle" size={16} className="flex-shrink-0 text-red-500" />
                    <span>{error}</span>
                  </p>
                )}

                <button 
                  type="submit" 
                  disabled={loading} 
                  className="w-full h-12 bg-blue-600 hover:bg-blue-700 text-white font-bold rounded-2xl shadow-lg shadow-blue-500/20 transition-all flex items-center justify-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed mt-4 text-sm"
                >
                  {loading ? (
                    <>
                      <div className="w-4 h-4 border-2 border-white/40 border-t-white rounded-full animate-spin" />
                      <span>Sending Verification Code...</span>
                    </>
                  ) : (
                    <>
                      <span>Create Owner Account</span>
                      <AppIcon name="ArrowRight" size={18} />
                    </>
                  )}
                </button>
              </motion.form>
            ) : (
              /* ─── Stage 2: OTP Verification Card ────────────────────────── */
              <motion.div
                key="otp-verification"
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -20 }}
                transition={{ duration: 0.2 }}
                className="space-y-5"
              >
                {/* Status Messages */}
                {!error && successMessage && (
                  <div className="text-xs text-emerald-700 font-semibold bg-emerald-50 p-3.5 rounded-2xl border border-emerald-200 flex items-center gap-2">
                    <AppIcon name="CheckCircle" size={16} className="flex-shrink-0 text-emerald-600" />
                    <span>{successMessage}</span>
                  </div>
                )}

                {error && (
                  <div className="text-[11px] text-red-600 font-bold bg-red-50/90 p-3.5 rounded-2xl border border-red-200 flex items-center gap-2 shadow-sm">
                    <AppIcon name="AlertCircle" size={16} className="flex-shrink-0 text-red-500" />
                    <span>{error}</span>
                  </div>
                )}

                {/* 6-Digit Segmented Input */}
                <div className="py-1">
                  <OtpInput
                    length={6}
                    value={otpValue}
                    onChange={(val: string) => {
                      setOtpValue(val);
                      setError("");
                    }}
                    onComplete={(code: string) => handleVerifyOTP(code)}
                    disabled={isVerifying}
                    hasError={Boolean(error)}
                    autoFocus
                  />
                </div>

                {/* Expiration Countdown & Change Email */}
                <div className="flex items-center justify-between px-1 text-xs">
                  <div className="flex items-center gap-1.5 font-semibold text-slate-600">
                    <AppIcon name="Clock" size={14} className={timeRemaining <= 60 ? "text-red-500" : "text-blue-500"} />
                    <span>Expires in:</span>
                    <span className={`font-mono font-bold ${timeRemaining <= 60 ? "text-red-600 animate-pulse" : "text-slate-900"}`}>
                      {formatTimer(timeRemaining)}
                    </span>
                  </div>

                  <button
                    type="button"
                    onClick={() => {
                      setNewEmailInput(pendingSession?.email || "");
                      setShowChangeEmail(prev => !prev);
                      setError("");
                    }}
                    className="text-blue-600 hover:text-blue-800 font-semibold hover:underline"
                  >
                    Change email
                  </button>
                </div>

                {/* Inline Change Email Form Drawer */}
                {showChangeEmail && (
                  <motion.form
                    initial={{ opacity: 0, height: 0 }}
                    animate={{ opacity: 1, height: "auto" }}
                    exit={{ opacity: 0, height: 0 }}
                    onSubmit={handleChangeEmailSubmit}
                    className="p-3.5 bg-blue-50/60 rounded-2xl border border-blue-100 space-y-2.5"
                  >
                    <label className="text-[11px] font-bold text-blue-900 uppercase tracking-wider block">
                      Update Verification Email
                    </label>
                    <div className="flex gap-2">
                      <input
                        type="email"
                        required
                        value={newEmailInput}
                        onChange={(e) => setNewEmailInput(e.target.value)}
                        placeholder="newowner@example.com"
                        className="flex-1 h-10 px-3 text-xs bg-white rounded-xl border border-blue-200 outline-none focus:border-blue-600 font-semibold"
                      />
                      <button
                        type="submit"
                        disabled={isChangingEmail}
                        className="px-4 h-10 bg-blue-600 hover:bg-blue-700 text-white font-bold text-xs rounded-xl disabled:opacity-50"
                      >
                        {isChangingEmail ? "Saving..." : "Update"}
                      </button>
                    </div>
                  </motion.form>
                )}

                {/* Primary Verification Action */}
                <button
                  type="button"
                  onClick={() => handleVerifyOTP()}
                  disabled={isVerifying || otpValue.length !== 6}
                  className="w-full h-12 bg-blue-600 hover:bg-blue-700 text-white font-bold rounded-2xl shadow-lg shadow-blue-500/20 transition-all flex items-center justify-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed text-sm"
                >
                  {isVerifying ? (
                    <>
                      <div className="w-4 h-4 border-2 border-white/40 border-t-white rounded-full animate-spin" />
                      <span>Verifying Code...</span>
                    </>
                  ) : (
                    <>
                      <span>Verify & Activate Account</span>
                      <AppIcon name="ArrowRight" size={18} />
                    </>
                  )}
                </button>

                {/* Resend OTP Action */}
                <div className="text-center pt-2">
                  {resendCooldown > 0 ? (
                    <p className="text-xs text-slate-400 font-medium">
                      Didn't receive code? Resend available in{" "}
                      <span className="font-mono font-bold text-slate-600">{resendCooldown}s</span>
                    </p>
                  ) : (
                    <button
                      type="button"
                      onClick={handleResendOTP}
                      disabled={isResending}
                      className="text-xs font-bold text-blue-600 hover:text-blue-800 hover:underline inline-flex items-center gap-1.5"
                    >
                      {isResending ? (
                        <span>Sending new code...</span>
                      ) : (
                        <>
                          <AppIcon name="RefreshCw" size={13} />
                          <span>Resend verification code</span>
                        </>
                      )}
                    </button>
                  )}
                </div>

                {/* Return to signup form */}
                <div className="text-center border-t border-slate-100 pt-3">
                  <button
                    type="button"
                    onClick={() => {
                      sessionStorage.removeItem(STORAGE_KEY);
                      setStage("form");
                      setError("");
                      setSuccessMessage("");
                    }}
                    className="text-xs text-slate-500 hover:text-slate-800 font-semibold"
                  >
                    ← Back to signup details
                  </button>
                </div>
              </motion.div>
            )}
          </AnimatePresence>

          {/* Footer SignIn link */}
          <div className="mt-6 text-center border-t border-gray-100 pt-5">
            <p className="text-gray-500 font-semibold text-xs tracking-wider">
              Already have an account?{" "}
              <Link to="/auth/login" className="text-blue-600 hover:underline underline-offset-4 font-bold">
                Sign In
              </Link>
            </p>
          </div>
        </motion.div>
      </div>
    </div>
  );
}

function Field({ label, name, value, onChange, type = "text", placeholder, required, autoFocus }: any) {
  return (
    <div className="space-y-1.5">
      <label className="text-[11px] font-bold text-gray-600 uppercase tracking-wider ml-1">{label}</label>
      <input 
        name={name} 
        type={type} 
        required={required} 
        value={value} 
        onChange={onChange} 
        placeholder={placeholder} 
        autoFocus={autoFocus} 
        className="w-full h-12 px-5 bg-white/70 border border-gray-200 rounded-2xl focus:bg-white focus:border-blue-600 focus:ring-4 focus:ring-blue-500/10 transition-all outline-none text-gray-900 font-semibold placeholder:text-gray-400 text-sm shadow-sm" 
      />
    </div>
  );
}

function PasswordField({ label, name, value, onChange, show, onToggle }: any) {
  return (
    <div className="space-y-1.5">
      <label className="text-[11px] font-bold text-gray-600 uppercase tracking-wider ml-1">{label}</label>
      <div className="relative">
        <input 
          name={name} 
          type={show ? "text" : "password"} 
          required 
          value={value} 
          onChange={onChange} 
          placeholder="••••••••" 
          className="w-full h-12 pl-5 pr-12 bg-white/70 border border-gray-200 rounded-2xl focus:bg-white focus:border-blue-600 focus:ring-4 focus:ring-blue-500/10 transition-all outline-none text-gray-900 font-semibold placeholder:text-gray-400 text-sm shadow-sm" 
        />
        <button 
          type="button" 
          onClick={onToggle} 
          className="absolute right-4 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 transition-colors" 
          tabIndex={-1}
        >
          {show ? <AppIcon name="EyeOff" size={18} /> : <AppIcon name="Eye" size={18} />}
        </button>
      </div>
    </div>
  );
}
