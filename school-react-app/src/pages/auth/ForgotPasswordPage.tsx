/**
 * ForgotPasswordPage.tsx
 * Multi-step, secure Forgot Password flow.
 * Steps: 1. Request OTP -> 2. Verify OTP -> 3. Set New Password -> 4. Success Redirect
 */

import { useState, useEffect, type FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { motion, AnimatePresence } from "framer-motion";
import { AppIcon } from "shared/ui/AppIcon";
import { OtpInput } from "@/components/auth/OtpInput";

type FlowStep = "REQUEST" | "VERIFY" | "RESET" | "SUCCESS";

export function ForgotPasswordPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();

  // Form State
  const [step, setStep] = useState<FlowStep>("REQUEST");
  const [email, setEmail] = useState("");
  const [otp, setOtp] = useState("");
  const [resetId, setResetId] = useState("");
  const [resetToken, setResetToken] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);

  // Status & Timers
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [infoMessage, setInfoMessage] = useState("");
  const [expirySeconds, setExpirySeconds] = useState(300);
  const [resendCooldown, setResendCooldown] = useState(60);
  const [attemptsRemaining, setAttemptsRemaining] = useState<number | null>(null);

  // Initialize email from URL if provided
  useEffect(() => {
    const emailParam = searchParams.get("email");
    if (emailParam) {
      setEmail(emailParam.trim());
    }
  }, [searchParams]);

  // Expiry countdown timer (Step 2)
  useEffect(() => {
    if (step !== "VERIFY" || expirySeconds <= 0) return;
    const timer = setInterval(() => {
      setExpirySeconds((prev) => Math.max(0, prev - 1));
    }, 1000);
    return () => clearInterval(timer);
  }, [step, expirySeconds]);

  // Resend cooldown timer (Step 2)
  useEffect(() => {
    if (step !== "VERIFY" || resendCooldown <= 0) return;
    const timer = setInterval(() => {
      setResendCooldown((prev) => Math.max(0, prev - 1));
    }, 1000);
    return () => clearInterval(timer);
  }, [step, resendCooldown]);

  // Format seconds to mm:ss
  const formatTime = (totalSeconds: number) => {
    const mins = Math.floor(totalSeconds / 60);
    const secs = totalSeconds % 60;
    return `${mins.toString().padStart(2, "0")}:${secs.toString().padStart(2, "0")}`;
  };

  // Password validation checks
  const hasMinLength = password.length >= 8;
  const hasLetter = /[a-zA-Z]/.test(password);
  const hasDigit = /[0-9]/.test(password);
  const passwordsMatch = password === confirmPassword && confirmPassword.length > 0;
  const isPasswordValid = hasMinLength && hasLetter && hasDigit && passwordsMatch;

  // Step 1: Request Password Reset OTP
  const handleRequestOTP = async (e?: FormEvent) => {
    if (e) e.preventDefault();
    const cleanEmail = email.trim().toLowerCase();
    if (!cleanEmail) {
      setError("Please enter your registered email address.");
      return;
    }

    setLoading(true);
    setError("");
    setInfoMessage("");

    try {
      const response = await fetch("/api/auth/forgot-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email: cleanEmail }),
      });

      const result = await response.json();
      if (!response.ok) {
        throw new Error(result?.message || result?.error?.message || "Failed to process password reset request.");
      }

      const data = result?.data || {};
      if (data.reset_id) {
        setResetId(data.reset_id);
      }
      setExpirySeconds(data.expires_in_seconds || 300);
      setResendCooldown(data.resend_cooldown_seconds || 60);
      setAttemptsRemaining(5);
      setStep("VERIFY");
    } catch (err: any) {
      setError(err?.message || "Something went wrong. Please check your internet connection and try again.");
    } finally {
      setLoading(false);
    }
  };

  // Step 2: Verify 6-digit OTP
  const handleVerifyOTP = async (codeToVerify?: string) => {
    const code = (codeToVerify || otp).trim();
    if (code.length !== 6) {
      setError("Please enter the complete 6-digit verification code.");
      return;
    }

    setLoading(true);
    setError("");
    setInfoMessage("");

    try {
      const response = await fetch("/api/auth/verify-reset-otp", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          reset_id: resetId,
          email: email.trim().toLowerCase(),
          otp: code,
        }),
      });

      const result = await response.json();
      if (!response.ok) {
        const errorMsg = result?.message || result?.error?.message || "Incorrect verification code.";
        // Parse attempts if available
        const match = errorMsg.match(/(\d+)\s+attempt/i);
        if (match && match[1]) {
          setAttemptsRemaining(parseInt(match[1], 10));
        }
        throw new Error(errorMsg);
      }

      const token = result?.data?.reset_token;
      if (!token) {
        throw new Error("Missing reset authorization token. Please try again.");
      }

      setResetToken(token);
      setStep("RESET");
    } catch (err: any) {
      setError(err?.message || "Failed to verify code.");
    } finally {
      setLoading(false);
    }
  };

  // Resend OTP
  const handleResendOTP = async () => {
    if (resendCooldown > 0 || loading) return;

    setLoading(true);
    setError("");
    setInfoMessage("");

    try {
      const response = await fetch("/api/auth/resend-reset-otp", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          reset_id: resetId,
          email: email.trim().toLowerCase(),
        }),
      });

      const result = await response.json();
      if (!response.ok) {
        throw new Error(result?.message || result?.error?.message || "Failed to resend code.");
      }

      const data = result?.data || {};
      if (data.reset_id) {
        setResetId(data.reset_id);
      }
      setExpirySeconds(data.expires_in_seconds || 300);
      setResendCooldown(data.resend_cooldown_seconds || 60);
      setOtp("");
      setAttemptsRemaining(5);
      setInfoMessage("A fresh 6-digit verification code has been dispatched to your email.");
    } catch (err: any) {
      setError(err?.message || "Unable to resend code right now.");
    } finally {
      setLoading(false);
    }
  };

  // Step 3: Set New Password
  const handleResetPassword = async (e: FormEvent) => {
    e.preventDefault();
    if (!isPasswordValid) {
      setError("Please ensure your password fulfills all security requirements.");
      return;
    }

    setLoading(true);
    setError("");

    try {
      const response = await fetch("/api/auth/reset-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          reset_token: resetToken,
          password: password.trim(),
          confirm_password: confirmPassword.trim(),
        }),
      });

      const result = await response.json();
      if (!response.ok) {
        throw new Error(result?.message || result?.error?.message || "Failed to update password.");
      }

      setStep("SUCCESS");
      // Auto-redirect to login after 3.5 seconds with email pre-filled
      setTimeout(() => {
        navigate(`/auth/login?email=${encodeURIComponent(email.trim().toLowerCase())}`);
      }, 3500);
    } catch (err: any) {
      setError(err?.message || "Password update failed. Please try again.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-[#f8fafc] relative flex items-center justify-center p-4 md:p-12 overflow-hidden">
      {/* Background with blur */}
      <div
        className="absolute inset-0 z-0 bg-cover bg-center bg-no-repeat"
        style={{ backgroundImage: 'url("/school-bg.png")' }}
      />
      <div className="absolute inset-0 z-0 bg-white/40 backdrop-blur-[2px]" />

      <div className="w-full max-w-[480px] relative z-10">
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          className="bg-white/70 backdrop-blur-2xl rounded-[40px] shadow-[0_40px_100px_rgba(0,0,0,0.1)] border border-white/40 p-8 md:p-10"
        >
          {/* Logo Header */}
          <div className="text-center mb-6">
            <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center overflow-hidden rounded-2xl bg-white/80 shadow-sm ring-1 ring-white/50">
              <img src="/logo.jpeg" alt="EduPlexo" className="h-full w-full object-cover" />
            </div>
          </div>

          <AnimatePresence mode="wait">
            {/* ─── STEP 1: REQUEST OTP ──────────────────────────────────────── */}
            {step === "REQUEST" && (
              <motion.div
                key="step-request"
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -20 }}
                transition={{ duration: 0.2 }}
              >
                <div className="text-center mb-6">
                  <h2 className="text-3xl font-black text-gray-900 mb-1.5 tracking-tight">Forgot Password?</h2>
                  <p className="text-gray-500 font-medium text-xs leading-relaxed px-4">
                    Enter your registered email address and we'll send you a 6-digit verification code to reset your password.
                  </p>
                </div>

                <form onSubmit={handleRequestOTP} className="space-y-5">
                  <div className="space-y-1.5">
                    <label className="text-[11px] font-bold text-gray-500 tracking-wide ml-2">Email Address</label>
                    <div className="relative">
                      <input
                        type="email"
                        required
                        value={email}
                        onChange={(e) => {
                          setEmail(e.target.value);
                          setError("");
                        }}
                        placeholder="admin@school.com"
                        autoFocus
                        className="w-full h-12 pl-12 pr-6 bg-white/50 border-2 border-transparent rounded-2xl focus:bg-white focus:border-blue-600 transition-all outline-none text-gray-900 font-bold placeholder:text-gray-300"
                      />
                      <div className="absolute left-4 top-1/2 -translate-y-1/2 text-gray-400">
                        <AppIcon name="Mail" size={18} />
                      </div>
                    </div>
                  </div>

                  {error && (
                    <p className="text-[11px] text-red-500 font-bold bg-red-50/80 p-4 rounded-2xl border border-red-100 flex items-center gap-2 shadow-sm">
                      <AppIcon name="AlertCircle" size={16} className="flex-shrink-0" />
                      {error}
                    </p>
                  )}

                  <button
                    type="submit"
                    disabled={loading}
                    className="w-full h-12 bg-blue-600 hover:bg-blue-700 text-white font-bold text-base rounded-2xl shadow-xl transition-all flex items-center justify-center gap-2 disabled:opacity-50"
                  >
                    {loading ? "Sending Code..." : "Send Verification Code"}
                    {!loading && <AppIcon name="ArrowRight" size={18} />}
                  </button>

                  <div className="pt-2 text-center">
                    <Link
                      to="/auth/login"
                      className="inline-flex items-center gap-2 text-xs font-bold text-gray-500 hover:text-gray-900 transition-colors"
                    >
                      <AppIcon name="ArrowLeft" size={14} />
                      Back to Sign In
                    </Link>
                  </div>
                </form>
              </motion.div>
            )}

            {/* ─── STEP 2: VERIFY OTP ───────────────────────────────────────── */}
            {step === "VERIFY" && (
              <motion.div
                key="step-verify"
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -20 }}
                transition={{ duration: 0.2 }}
              >
                <div className="text-center mb-4">
                  <h2 className="text-3xl font-black text-gray-900 mb-1 tracking-tight">Enter Code</h2>
                  <p className="text-gray-500 font-medium text-xs leading-relaxed px-2">
                    We sent a 6-digit verification code to:
                  </p>
                  <div className="mt-1 flex items-center justify-center gap-2">
                    <span className="font-bold text-gray-900 text-xs bg-white/70 px-3 py-1 rounded-full border border-gray-200 shadow-sm">
                      {email}
                    </span>
                    <button
                      type="button"
                      onClick={() => {
                        setStep("REQUEST");
                        setError("");
                        setOtp("");
                      }}
                      className="text-[11px] font-bold text-blue-600 hover:underline"
                    >
                      Change
                    </button>
                  </div>
                </div>

                <div className="space-y-4">
                  <div className="flex flex-col items-center">
                    <OtpInput
                      value={otp}
                      length={6}
                      onChange={(val) => {
                        setOtp(val);
                        setError("");
                      }}
                      onComplete={(code) => handleVerifyOTP(code)}
                      disabled={loading}
                      hasError={Boolean(error)}
                    />

                    {/* Expiry and attempts badge */}
                    <div className="flex items-center justify-between w-full px-2 text-[11px] font-semibold text-gray-400">
                      <span>
                        Expires in:{" "}
                        <span className={`font-mono font-bold ${expirySeconds < 60 ? "text-red-500" : "text-gray-700"}`}>
                          {formatTime(expirySeconds)}
                        </span>
                      </span>
                      {attemptsRemaining !== null && (
                        <span className={`${attemptsRemaining <= 2 ? "text-amber-600 font-bold" : "text-gray-400"}`}>
                          {attemptsRemaining} {attemptsRemaining === 1 ? "attempt" : "attempts"} left
                        </span>
                      )}
                    </div>
                  </div>

                  {error && (
                    <p className="text-[11px] text-red-500 font-bold bg-red-50/80 p-3.5 rounded-2xl border border-red-100 flex items-center gap-2 shadow-sm">
                      <AppIcon name="AlertCircle" size={16} className="flex-shrink-0" />
                      {error}
                    </p>
                  )}

                  {infoMessage && (
                    <p className="text-[11px] text-green-700 font-semibold bg-green-50 p-3 rounded-xl border border-green-200 text-center">
                      {infoMessage}
                    </p>
                  )}

                  <button
                    type="button"
                    onClick={() => handleVerifyOTP()}
                    disabled={loading || otp.length !== 6 || expirySeconds <= 0}
                    className="w-full h-12 bg-blue-600 hover:bg-blue-700 text-white font-bold text-base rounded-2xl shadow-xl transition-all flex items-center justify-center gap-2 disabled:opacity-50"
                  >
                    {loading ? "Verifying Code..." : "Verify Code"}
                    {!loading && <AppIcon name="ArrowRight" size={18} />}
                  </button>

                  {/* Resend Action */}
                  <div className="pt-2 text-center flex flex-col items-center gap-2">
                    <button
                      type="button"
                      onClick={handleResendOTP}
                      disabled={resendCooldown > 0 || loading}
                      className="text-xs font-bold text-blue-600 disabled:text-gray-400 hover:underline transition-colors disabled:no-underline"
                    >
                      {resendCooldown > 0
                        ? `Resend code in ${resendCooldown}s`
                        : "Didn't receive code? Resend"}
                    </button>

                    <Link
                      to="/auth/login"
                      className="inline-flex items-center gap-1.5 text-xs font-bold text-gray-400 hover:text-gray-700 transition-colors mt-2"
                    >
                      <AppIcon name="ArrowLeft" size={14} />
                      Back to Sign In
                    </Link>
                  </div>
                </div>
              </motion.div>
            )}

            {/* ─── STEP 3: RESET PASSWORD ───────────────────────────────────── */}
            {step === "RESET" && (
              <motion.div
                key="step-reset"
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -20 }}
                transition={{ duration: 0.2 }}
              >
                <div className="text-center mb-6">
                  <h2 className="text-3xl font-black text-gray-900 mb-1 tracking-tight">Create New Password</h2>
                  <p className="text-gray-500 font-medium text-xs">
                    Choose a secure password for your EduPlexo account.
                  </p>
                </div>

                <form onSubmit={handleResetPassword} className="space-y-4">
                  <div className="space-y-1.5">
                    <label className="text-[11px] font-bold text-gray-500 tracking-wide ml-2">New Password</label>
                    <div className="relative">
                      <input
                        type={showPassword ? "text" : "password"}
                        required
                        value={password}
                        onChange={(e) => {
                          setPassword(e.target.value);
                          setError("");
                        }}
                        placeholder="••••••••"
                        autoFocus
                        className="w-full h-12 pl-6 pr-12 bg-white/50 border-2 border-transparent rounded-2xl focus:bg-white focus:border-blue-600 transition-all outline-none text-gray-900 font-bold placeholder:text-gray-300"
                      />
                      <button
                        type="button"
                        onClick={() => setShowPassword(!showPassword)}
                        className="absolute right-4 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 transition-colors"
                        tabIndex={-1}
                      >
                        <AppIcon name={showPassword ? "EyeOff" : "Eye"} size={18} />
                      </button>
                    </div>
                  </div>

                  <div className="space-y-1.5">
                    <label className="text-[11px] font-bold text-gray-500 tracking-wide ml-2">Confirm Password</label>
                    <div className="relative">
                      <input
                        type={showConfirmPassword ? "text" : "password"}
                        required
                        value={confirmPassword}
                        onChange={(e) => {
                          setConfirmPassword(e.target.value);
                          setError("");
                        }}
                        placeholder="••••••••"
                        className="w-full h-12 pl-6 pr-12 bg-white/50 border-2 border-transparent rounded-2xl focus:bg-white focus:border-blue-600 transition-all outline-none text-gray-900 font-bold placeholder:text-gray-300"
                      />
                      <button
                        type="button"
                        onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                        className="absolute right-4 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 transition-colors"
                        tabIndex={-1}
                      >
                        <AppIcon name={showConfirmPassword ? "EyeOff" : "Eye"} size={18} />
                      </button>
                    </div>
                  </div>

                  {/* Password Requirements Checklist */}
                  <div className="p-3.5 bg-white/60 rounded-2xl border border-gray-200/80 space-y-2 text-[11px]">
                    <div className="flex items-center gap-2">
                      <div className={`h-4 w-4 rounded-full flex items-center justify-center ${hasMinLength ? "bg-green-100 text-green-600" : "bg-gray-100 text-gray-400"}`}>
                        <AppIcon name={hasMinLength ? "Check" : "Lock"} size={10} />
                      </div>
                      <span className={hasMinLength ? "text-green-700 font-bold" : "text-gray-500"}>
                        At least 8 characters
                      </span>
                    </div>
                    <div className="flex items-center gap-2">
                      <div className={`h-4 w-4 rounded-full flex items-center justify-center ${hasLetter && hasDigit ? "bg-green-100 text-green-600" : "bg-gray-100 text-gray-400"}`}>
                        <AppIcon name={hasLetter && hasDigit ? "Check" : "Lock"} size={10} />
                      </div>
                      <span className={hasLetter && hasDigit ? "text-green-700 font-bold" : "text-gray-500"}>
                        Contains letters and numbers
                      </span>
                    </div>
                    {confirmPassword.length > 0 && (
                      <div className="flex items-center gap-2">
                        <div className={`h-4 w-4 rounded-full flex items-center justify-center ${passwordsMatch ? "bg-green-100 text-green-600" : "bg-red-100 text-red-500"}`}>
                          <AppIcon name={passwordsMatch ? "Check" : "AlertCircle"} size={10} />
                        </div>
                        <span className={passwordsMatch ? "text-green-700 font-bold" : "text-red-500 font-bold"}>
                          {passwordsMatch ? "Passwords match" : "Passwords do not match"}
                        </span>
                      </div>
                    )}
                  </div>

                  {error && (
                    <p className="text-[11px] text-red-500 font-bold bg-red-50/80 p-4 rounded-2xl border border-red-100 flex items-center gap-2 shadow-sm">
                      <AppIcon name="AlertCircle" size={16} className="flex-shrink-0" />
                      {error}
                    </p>
                  )}

                  <button
                    type="submit"
                    disabled={loading || !isPasswordValid}
                    className="w-full h-12 bg-blue-600 hover:bg-blue-700 text-white font-bold text-base rounded-2xl shadow-xl transition-all flex items-center justify-center gap-2 disabled:opacity-50"
                  >
                    {loading ? "Updating Password..." : "Reset Password"}
                    {!loading && <AppIcon name="ArrowRight" size={18} />}
                  </button>
                </form>
              </motion.div>
            )}

            {/* ─── STEP 4: SUCCESS ──────────────────────────────────────────── */}
            {step === "SUCCESS" && (
              <motion.div
                key="step-success"
                initial={{ opacity: 0, scale: 0.95 }}
                animate={{ opacity: 1, scale: 1 }}
                className="text-center py-4 space-y-5"
              >
                <div className="mx-auto h-20 w-20 rounded-full bg-green-100 flex items-center justify-center text-green-600 shadow-sm ring-8 ring-green-50">
                  <AppIcon name="CheckCircle2" size={42} />
                </div>

                <div className="space-y-1">
                  <h2 className="text-2xl font-black text-gray-900 tracking-tight">Password Reset Complete!</h2>
                  <p className="text-gray-500 text-xs font-medium max-w-xs mx-auto leading-relaxed">
                    Your password has been changed successfully. You can now log in with your new credentials.
                  </p>
                </div>

                <div className="pt-2">
                  <button
                    type="button"
                    onClick={() => navigate(`/auth/login?email=${encodeURIComponent(email.trim().toLowerCase())}`)}
                    className="w-full h-12 bg-blue-600 hover:bg-blue-700 text-white font-bold text-base rounded-2xl shadow-xl transition-all flex items-center justify-center gap-2"
                  >
                    Proceed to Sign In
                    <AppIcon name="ArrowRight" size={18} />
                  </button>
                </div>

                <p className="text-[11px] font-bold text-gray-400">
                  Redirecting automatically in a few seconds...
                </p>
              </motion.div>
            )}
          </AnimatePresence>
        </motion.div>
      </div>
    </div>
  );
}
