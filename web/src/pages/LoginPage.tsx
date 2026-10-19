import { Alert, Box, Button, Link, Paper, Stack, TextField, Typography } from '@mui/material';
import { useEffect, useState } from 'react';
import { errorMessage, get } from '../api/client';
import { useAuth } from '../auth/AuthContext';

export function LoginPage() {
  const { login, register } = useAuth();
  const [signupOpen, setSignupOpen] = useState(false);
  const [mode, setMode] = useState<'login' | 'register'>('login');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    get<{ signup_open: boolean }>('/auth/config')
      .then((c) => setSignupOpen(c.signup_open))
      .catch(() => setSignupOpen(false));
  }, []);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      await (mode === 'login' ? login(email, password) : register(email, password));
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Box sx={{ minHeight: '100vh', display: 'grid', placeItems: 'center', bgcolor: 'background.default', p: 2 }}>
      <Paper variant="outlined" sx={{ p: 4, width: '100%', maxWidth: 380 }}>
        <Typography variant="h4" sx={{ fontWeight: 800, color: 'primary.main', letterSpacing: -1 }}>
          kala
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
          Tasks, tags and time — from the keyboard.
        </Typography>
        <form onSubmit={submit}>
          <Stack spacing={2}>
            {error && <Alert severity="error">{error}</Alert>}
            <TextField label="Email" type="email" autoComplete="email" autoFocus required value={email} onChange={(e) => setEmail(e.target.value)} />
            <TextField
              label="Password"
              type="password"
              autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              helperText={mode === 'register' ? 'At least 8 characters' : undefined}
            />
            <Button type="submit" variant="contained" disabled={busy}>
              {mode === 'login' ? 'Sign in' : 'Create account'}
            </Button>
            {signupOpen && (
              <Typography variant="body2" sx={{ textAlign: 'center' }}>
                {mode === 'login' ? 'No account yet? ' : 'Already registered? '}
                <Link component="button" type="button" onClick={() => setMode(mode === 'login' ? 'register' : 'login')}>
                  {mode === 'login' ? 'Create one' : 'Sign in'}
                </Link>
              </Typography>
            )}
          </Stack>
        </form>
      </Paper>
    </Box>
  );
}
