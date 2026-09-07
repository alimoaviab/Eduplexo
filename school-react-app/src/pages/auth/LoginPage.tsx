/**
 * Login page.
 */

import { useEffect, useState, type ChangeEvent, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { motion } from "framer-motion";
import { AppIcon } from "shared/ui/AppIcon";
import { decodeJwtPayload } from "@/utils/jwt";

type Role = "admin" | "teacher" | "student";

const ROLE_ICONS: Record<Role, React.ReactNode> = {
  admin: <AppIcon name="ShieldCheck" size={20} />,
  teacher: <AppIcon name="BookOpen" size={20} />,
  student: <AppIcon name="GraduationCap" size={20} />,
};

const ROLES: { key: Role; label: string }[] = [
  { key: "admin", label: "Admin" },
  { key: "teacher", label: "Teacher" },
  { key: "student", label: "Student" },
];

const ROLE_ROUTES: Record<string, string> = {
  admin: "/admin/dashboard",
  super_admin: "/admin/dashboard",
  teacher: "/teacher/dashboard",
  student: "/student/dashboard",
};

function resolveRoleRoute(role?: string): string {
  const normalizedRole = (role || "").toLowerCase();
  return ROLE_ROUTES[normalizedRole] || "/admin/dashboard";
}


function suggestedTabForRole(role: string): Role {
  const r = role.toLowerCase();
  if (r === "teacher") return "teacher";
  if (r === "student") return "student";
  return "admin";
}


export function LoginPage() {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(false);
  const [sessionChecked, setSessionChecked] = useState(false);
  const [selectedRole, setSelectedRole] = useState<Role>("admin");
  const [formData, setFormData] = useState({ email: "", password: "", rememberMe: false });
  const [showPassword, setShowPassword] = useState(false);

  useEffect(() => {
    const token = typeof window !== "undefined" ? window.localStorage.getItem("token") : null;
    if (token) {
      const payload = decodeJwtPayload(token);
      navigate(resolveRoleRoute(payload?.role), { replace: true });
      return;
    }
    setSessionChecked(true);
  }, [navigate]);

  if (!sessionChecked) return null;

  const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    const { name, value, type, checked } = e.target;
    setFormData((prev) => ({ ...prev, [name]: type === "checkbox" ? checked : value }));
    setError("");
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLFormElement>) => {
    if (e.key === "Enter") {
      const target = e.target as HTMLElement;
      if (target.tagName === "INPUT") {
        e.preventDefault();
        const form = target.closest("form");
        if (!form) return;
        
        const focusables = Array.from(
          form.querySelectorAll("input:not([disabled])")
        ) as HTMLElement[];
        
        const index = focusables.indexOf(target);
        if (index > -1 && index < focusables.length - 1) {
          focusables[index + 1].focus();
        } else {
          handleSubmit(e as unknown as FormEvent<HTMLFormElement>);
        }
      }
    }
  };

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setLoading(true);
    setError("");

    try {
      const response = await fetch("/api/auth/login", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...formData, role: selectedRole }),
      });

      const raw = await response.text();
      let result: any = null;
      if (raw) {
        try {
          result = JSON.parse(raw);
        } catch {
          result = null;
        }
      }

      const payload = result?.data && typeof result.data === "object" ? result.data : result;

      if (!response.ok) {
        let message = result?.message || "Invalid email or password.";
        const errCode = result?.error?.code as string | undefined;
        const isSuspended = errCode === "ACCOUNT_SUSPENDED" || errCode === "SUBSCRIPTION_SUSPENDED";
        
        // Security: Never reveal internal roles or account existence in error messages (OWASP CWE-209)
        if (!isSuspended) {
          if (
            response.status === 401 ||
            response.status === 403 ||
            /registered as/i.test(message) ||
            /tab to sign in/i.test(message) ||
            errCode === "ROLE_MISMATCH"
          ) {
            message = "Invalid email or password.";
          }
        }
        throw new Error(message);
      }

      // Smoothly sync selected tab if role differs, without interrupting login
      const actualRole = (payload?.role as string | undefined)?.toLowerCase() || "";
      if (actualRole) {
        const matchingTab = suggestedTabForRole(actualRole);
        if (matchingTab !== selectedRole) {
          setSelectedRole(matchingTab);
        }
      }


      setSuccess(true);
      if (payload?.token) {
        // Clear prior session tokens to prevent bleed
        localStorage.removeItem("active_school_id");
        localStorage.removeItem("active_branch_id");
        localStorage.removeItem("academic_year_id");
        localStorage.removeItem("last_school_id");
        localStorage.removeItem("profile_id");
        localStorage.removeItem("class_id");
        localStorage.removeItem("student_id");

        localStorage.setItem("token", payload.token);
        if (payload?.school_id && payload.school_id !== "system") {
          localStorage.setItem("active_school_id", payload.school_id);
        }
        window.dispatchEvent(new Event("auth-changed"));
      }

      setTimeout(() => {
        const targetRole = payload?.role || selectedRole;
        navigate(resolveRoleRoute(targetRole));
      }, 1500);
    } catch (err: unknown) {
      setError((err as Error).message || "Sign in failed.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-[#f8fafc] relative flex items-center justify-center p-4 md:p-12 overflow-hidden">
      {/* Aesthetic Background */}
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
          <div className="text-center mb-8">
            <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center overflow-hidden rounded-2xl bg-white/80 shadow-sm ring-1 ring-white/50">
              <img src="/logo.jpeg" alt="Eduplexo" className="h-full w-full object-cover" />
            </div>
            <h2 className="text-4xl font-black text-gray-900 mb-1 tracking-tight">Welcome Back</h2>
            <p className="text-gray-500 font-bold text-xs tracking-wide">Sign in to continue</p>
          </div>

          <div className="mb-8 p-1.5 bg-white/30 backdrop-blur-md rounded-2xl flex border border-white/40">
            {ROLES.map((role) => (
              <button
                key={role.key}
                type="button"
                onClick={() => {
                  setSelectedRole(role.key);
                  setError("");
                }}
                className={`flex-1 py-4 px-2 rounded-xl text-[10px] font-black transition-all duration-300 flex flex-col items-center justify-center gap-1 relative ${
                  selectedRole === role.key ? "text-blue-600 bg-white shadow-sm ring-1 ring-white/50" : "text-gray-400 hover:text-gray-600"
                }`}
              >
                <span className="w-5 h-5 flex items-center justify-center flex-shrink-0">{ROLE_ICONS[role.key]}</span>
                <span className="tracking-wide">{role.label}</span>
              </button>
            ))}
          </div>

          <form className="space-y-6" onSubmit={handleSubmit} onKeyDown={handleKeyDown}>
            <div className="space-y-1.5">
              <label className="text-[11px] font-bold text-gray-500 tracking-wide ml-2">Email Address</label>
              <input name="email" type="email" required value={formData.email} onChange={handleChange} placeholder="name@school.com" autoFocus className="w-full h-12 px-6 bg-white/50 border-2 border-transparent rounded-2xl focus:bg-white focus:border-blue-600 transition-all outline-none text-gray-900 font-bold placeholder:text-gray-300" />
            </div>

            <div className="space-y-1.5">
              <label className="text-[11px] font-bold text-gray-500 tracking-wide ml-2">Password</label>
              <div className="relative">
                <input name="password" type={showPassword ? "text" : "password"} required value={formData.password} onChange={handleChange} placeholder="••••••••" className="w-full h-12 pl-6 pr-14 bg-white/50 border-2 border-transparent rounded-2xl focus:bg-white focus:border-blue-600 transition-all outline-none text-gray-900 font-bold placeholder:text-gray-300" />
                <button type="button" onClick={() => setShowPassword(!showPassword)} className="absolute right-4 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 transition-colors" tabIndex={-1}>
                  {showPassword ? <AppIcon name="EyeOff" size={20} /> : <AppIcon name="Eye" size={20} />}
                </button>
              </div>
            </div>

            {error && <p className="text-[11px] text-red-500 font-bold bg-red-50/80 p-4 rounded-2xl border border-red-100 flex items-center gap-2 shadow-sm"><AppIcon name="AlertCircle" size={16}  className="flex-shrink-0" />{error}</p>}

            <div className="flex items-center gap-2 ml-2">
              <input
                id="rememberMe"
                name="rememberMe"
                type="checkbox"
                checked={formData.rememberMe}
                onChange={handleChange}
                className="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500 cursor-pointer"
              />
              <label htmlFor="rememberMe" className="text-[11px] font-bold text-gray-500 cursor-pointer select-none">
                Remember me for 30 days
              </label>
            </div>

            <button type="submit" disabled={loading || success} className={`w-full h-12 bg-blue-600 hover:bg-blue-700 text-white font-bold text-lg rounded-2xl shadow-xl transition-all flex items-center justify-center gap-2 disabled:opacity-50 ${success ? "bg-green-600" : ""}`}>
              {loading ? "Authenticating..." : success ? "Welcome Back!" : "Sign In"}
              {!loading && !success && <AppIcon name="ArrowRight" size={20} />}
              {success && <AppIcon name="CheckCircle" size={20} />}
            </button>
          </form>

          <div className="mt-10 text-center">
            <p className="text-gray-500 font-bold text-xs tracking-wide">
              New member? <Link to="/auth/signup" className="text-blue-600 hover:underline underline-offset-4 decoration-2">Create Account</Link>
            </p>
          </div>
        </motion.div>
      </div>
    </div>
  );
}
