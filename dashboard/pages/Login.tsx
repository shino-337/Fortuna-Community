
import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuthStore } from '../store/authStore';
import { api } from '../lib/api';
import { ShieldAlert } from 'lucide-react';
import { Button } from '../components/ui/Button';

export const Login: React.FC = () => {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');
  const login = useAuthStore((state) => state.login);
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError('');

    try {
      const { user, token } = await api.login(username, password);
      login(user, token);
      navigate('/');
    } catch (err) {
      setError('Invalid credentials. Try any email not containing "error".');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-slate-950 flex flex-col items-center justify-center p-4">
      <div className="w-full max-w-md bg-slate-900 rounded-2xl shadow-xl p-8 border border-slate-800">
        <div className="flex flex-col items-center mb-8">
          <div className="w-12 h-12 bg-pink-600 rounded-xl flex items-center justify-center mb-4">
            <ShieldAlert className="text-white w-7 h-7" />
          </div>
          <h1 className="text-2xl font-bold text-white">Welcome back</h1>
          <p className="text-slate-400 mt-2">Sign in to access Fortuna</p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-6">
          {error && (
            <div className="p-3 bg-red-900/20 text-red-400 text-sm rounded-lg border border-red-900/50">
              {error}
            </div>
          )}

          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1">
              Username or Email
            </label>
            <input
              type="text"
              required
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="w-full px-4 py-2 bg-slate-950 border border-slate-700 rounded-lg text-white focus:ring-2 focus:ring-pink-500 focus:border-pink-500 outline-none transition-all placeholder:text-slate-600"
              placeholder="admin"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1">
              Password
            </label>
            <input
              type="password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full px-4 py-2 bg-slate-950 border border-slate-700 rounded-lg text-white focus:ring-2 focus:ring-pink-500 focus:border-pink-500 outline-none transition-all placeholder:text-slate-600"
              placeholder="••••••••"
            />
          </div>

          <Button 
            type="submit" 
            className="w-full py-3" 
            isLoading={isLoading}
          >
            Sign In
          </Button>

          <div className="text-center text-sm text-slate-500">
            <p>Use your Fortuna username or email.</p>
          </div>
        </form>
      </div>
    </div>
  );
};
