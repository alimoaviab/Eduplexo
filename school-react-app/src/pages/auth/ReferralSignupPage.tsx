import { useState, useEffect, type ChangeEvent, type FormEvent } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { AppIcon } from "shared/ui/AppIcon";
import { apiRequest } from "@/lib/api";
import { useAuth } from "@/hooks/useAuth";

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const PASSWORD_REGEX = /^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)(?=.*[^A-Za-z0-9\s]).{8,}$/;

interface ReferralOffer {
  plan_id: string;
  plan_name_snapshot: string;
  monthly_price_snapshot: number;
  currency: string;
  billing_period: string;
  publisher_name: string;
}

export function ReferralSignupPage() {
  const { token } = useParams<{ token: string }>();
  const navigate = useNavigate();
  const { verifySession } = useAuth();

  const [loadingOffer, setLoadingOffer] = useState(true);
  const [offerError, setOfferError] = useState("");
  const [offer, setOffer] = useState<ReferralOffer | null>(null);

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  
  const [formData, setFormData] = useState({
    schoolName: "",
    fullName: "",
    email: "",
    phone: "",
    password: "",
  });

  useEffect(() => {
    const fetchOffer = async () => {
      setLoadingOffer(true);
      const res = await apiRequest(`/api/referral/validate/${token}`);
      if (res.ok && res.data) {
        setOffer(res.data);
      } else {
        setOfferError(res.message || "This referral link is invalid or has expired.");
      }
      setLoadingOffer(false);
    };
    if (token) {
      fetchOffer();
    } else {
      setOfferError("No referral token provided.");
      setLoadingOffer(false);
    }
  }, [token]);

  function handleChange(e: ChangeEvent<HTMLInputElement>) {
    const { name, value } = e.target;
    setFormData(prev => ({ ...prev, [name]: value }));
    setError("");
  }

  function validate(): string | null {
    if (!formData.schoolName.trim()) return "School name is required";
    if (!formData.fullName.trim()) return "Administrator name is required";
    if (!formData.phone.trim()) return "Phone number is required";
    if (!formData.email.trim()) return "Email is required";
    if (!EMAIL_REGEX.test(formData.email)) return "Please enter a valid email address";
    if (!formData.password) return "Password is required";
    if (!PASSWORD_REGEX.test(formData.password)) return "Password must be 8+ chars with uppercase, lowercase, number, and special character";
    return null;
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const v = validate();
    if (v) {
      setError(v);
      return;
    }

    setLoading(true);
    setError("");

    const response = await fetch("/api/auth/signup/referral", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ 
        token,
        schoolName: formData.schoolName.trim(),
        fullName: formData.fullName.trim(),
        phone: formData.phone.trim(),
        email: formData.email.trim(),
        password: formData.password,
      }),
    });

    const data = await response.json();
    if (response.ok && data.ok) {
      // Direct signup bypasses OTP for referrals
      await verifySession();
      navigate("/");
    } else {
      setError(data.message || "Signup failed. Please try again.");
      setLoading(false);
    }
  }

  if (loadingOffer) {
    return (
      <div className="min-h-screen bg-slate-50 flex items-center justify-center p-4">
        <div className="text-center text-slate-500">Validating your referral link...</div>
      </div>
    );
  }

  if (offerError) {
    return (
      <div className="min-h-screen bg-slate-50 flex flex-col items-center justify-center p-4">
        <div className="bg-white p-8 rounded-2xl shadow-sm border border-slate-200 max-w-md w-full text-center">
          <div className="w-16 h-16 bg-red-100 rounded-full flex items-center justify-center mx-auto mb-4">
            <AppIcon name="alert-circle" className="h-8 w-8 text-red-600" />
          </div>
          <h2 className="text-xl font-bold text-slate-900 mb-2">Invalid Link</h2>
          <p className="text-slate-500 mb-6">{offerError}</p>
          <button onClick={() => navigate("/login")} className="w-full bg-slate-900 text-white font-medium py-2.5 rounded-lg">
            Return to Login
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-slate-50 flex flex-col items-center justify-center p-4">
      <div className="w-full max-w-4xl grid grid-cols-1 md:grid-cols-2 gap-8 items-start">
        
        {/* Offer Details */}
        <div className="bg-white rounded-2xl p-8 border border-slate-200 shadow-sm md:sticky md:top-8">
          <div className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-blue-50 text-blue-700 text-xs font-semibold mb-6">
            <AppIcon name="star" className="h-3.5 w-3.5" />
            Special Partner Offer
          </div>
          
          <h1 className="text-3xl font-bold text-slate-900 tracking-tight mb-2">
            You've been invited by <span className="text-blue-600">{offer?.publisher_name}</span>
          </h1>
          <p className="text-slate-500 mb-8">
            Complete your registration to instantly unlock EduPlexo for your school with your exclusive locked pricing.
          </p>

          <div className="bg-slate-50 rounded-xl p-6 border border-slate-100 mb-6">
            <div className="text-sm font-semibold text-slate-500 uppercase tracking-wider mb-1">Your Locked Plan</div>
            <div className="text-xl font-bold text-slate-900 mb-4">{offer?.plan_name_snapshot}</div>
            
            <div className="text-sm font-semibold text-slate-500 uppercase tracking-wider mb-1">Locked Pricing</div>
            <div className="flex items-baseline gap-1">
              <span className="text-3xl font-bold text-slate-900">
                {offer?.currency === 'PKR' ? 'Rs.' : offer?.currency} {offer?.monthly_price_snapshot.toLocaleString()}
              </span>
              <span className="text-slate-500 font-medium">/{offer?.billing_period}</span>
            </div>
          </div>
          
          <ul className="space-y-3">
            {[
              "Instant access to your school portal",
              "Unlimited student & staff accounts within limit",
              "Priority support & onboarding",
              "Price locked permanently for your subscription"
            ].map((feature, i) => (
              <li key={i} className="flex items-start gap-2.5">
                <AppIcon name="check-circle" className="h-5 w-5 text-green-500 shrink-0" />
                <span className="text-slate-700 font-medium text-sm">{feature}</span>
              </li>
            ))}
          </ul>
        </div>

        {/* Signup Form */}
        <div className="bg-white rounded-2xl p-8 border border-slate-200 shadow-sm">
          <h2 className="text-xl font-bold text-slate-900 mb-6">Create your Admin Account</h2>
          
          {error && (
            <div className="mb-6 p-4 bg-red-50 text-red-700 text-sm font-medium rounded-xl border border-red-100 flex items-start gap-2">
              <AppIcon name="alert-circle" className="h-5 w-5 shrink-0" />
              <span>{error}</span>
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-slate-700 mb-1.5">School / Institution Name *</label>
              <input
                name="schoolName"
                value={formData.schoolName}
                onChange={handleChange}
                className="w-full px-4 py-2.5 rounded-xl border border-slate-200 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500"
                placeholder="e.g. Springfield High"
                required
              />
            </div>
            
            <div>
              <label className="block text-sm font-medium text-slate-700 mb-1.5">Administrator Full Name *</label>
              <input
                name="fullName"
                value={formData.fullName}
                onChange={handleChange}
                className="w-full px-4 py-2.5 rounded-xl border border-slate-200 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500"
                placeholder="e.g. John Doe"
                required
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-slate-700 mb-1.5">Phone Number *</label>
              <input
                name="phone"
                value={formData.phone}
                onChange={handleChange}
                className="w-full px-4 py-2.5 rounded-xl border border-slate-200 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500"
                placeholder="e.g. +923001234567"
                required
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-slate-700 mb-1.5">Admin Email *</label>
              <input
                type="email"
                name="email"
                value={formData.email}
                onChange={handleChange}
                className="w-full px-4 py-2.5 rounded-xl border border-slate-200 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500"
                placeholder="admin@school.com"
                required
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-slate-700 mb-1.5">Create Password *</label>
              <input
                type="password"
                name="password"
                value={formData.password}
                onChange={handleChange}
                className="w-full px-4 py-2.5 rounded-xl border border-slate-200 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500"
                placeholder="At least 8 characters"
                required
              />
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full bg-blue-600 hover:bg-blue-700 text-white font-semibold py-3 rounded-xl transition-colors mt-6 disabled:opacity-50"
            >
              {loading ? "Creating Account..." : "Accept Offer & Register"}
            </button>
            
            <p className="text-xs text-center text-slate-500 mt-4">
              By registering, you agree to our Terms of Service and Privacy Policy.
            </p>
          </form>
        </div>
      </div>
    </div>
  );
}
