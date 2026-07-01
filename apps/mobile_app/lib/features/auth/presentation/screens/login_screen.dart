import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../providers/auth_provider.dart';
import '../widgets/auth_text_field.dart';

class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});

  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen> {
  final _formKey      = GlobalKey<FormState>();
  final _emailCtrl    = TextEditingController();
  final _passwordCtrl = TextEditingController();
  bool _isLoading     = false;
  bool _showWakeHint  = false;
  Timer? _wakeTimer;

  @override
  void dispose() {
    _wakeTimer?.cancel();
    _emailCtrl.dispose();
    _passwordCtrl.dispose();
    super.dispose();
  }

  Future<void> _login() async {
    if (!_formKey.currentState!.validate()) return;

    setState(() { _isLoading = true; _showWakeHint = false; });

    // Show hint after 4s so user knows the server is waking up
    _wakeTimer = Timer(const Duration(seconds: 4), () {
      if (mounted && _isLoading) setState(() => _showWakeHint = true);
    });

    try {
      await ref.read(authProvider.notifier).login(
        email:    _emailCtrl.text.trim(),
        password: _passwordCtrl.text,
      );
    } finally {
      _wakeTimer?.cancel();
      if (mounted) setState(() { _isLoading = false; _showWakeHint = false; });
    }
  }

  void _snack(String msg, {bool error = true}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: Text(msg, style: const TextStyle(color: Colors.white)),
      backgroundColor: error ? const Color(0xFFEE1D52) : const Color(0xFF2ECC71),
      behavior: SnackBarBehavior.floating,
    ));
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(authProvider, (_, next) {
      next.whenData((state) {
        if (state is AuthAuthenticated) {
          context.go('/home');
        } else if (state is AuthError) {
          _snack(state.message);
          ref.read(authProvider.notifier).clearError();
        }
      });
    });

    return Scaffold(
      backgroundColor: Colors.black,
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.symmetric(horizontal: 24),
          child: Form(
            key: _formKey,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.center,
              children: [
                const SizedBox(height: 60),

                const Text('TikTok',
                    style: TextStyle(color: Colors.white, fontSize: 40,
                        fontWeight: FontWeight.w900, letterSpacing: -1)),
                const SizedBox(height: 8),
                Text('Sign in to continue',
                    style: TextStyle(color: Colors.grey[500], fontSize: 14)),
                const SizedBox(height: 48),

                AuthTextField(
                  controller: _emailCtrl,
                  label: 'Email address',
                  hint: 'you@example.com',
                  keyboardType: TextInputType.emailAddress,
                  prefixIcon: const Icon(Icons.email_outlined),
                  validator: (v) {
                    if (v == null || v.trim().isEmpty) return 'Email required';
                    if (!v.contains('@')) return 'Enter a valid email';
                    return null;
                  },
                ),
                const SizedBox(height: 16),

                AuthTextField(
                  controller: _passwordCtrl,
                  label: 'Password',
                  isPassword: true,
                  prefixIcon: const Icon(Icons.lock_outline),
                  validator: (v) {
                    if (v == null || v.isEmpty) return 'Password required';
                    if (v.length < 6) return 'At least 6 characters';
                    return null;
                  },
                ),
                const SizedBox(height: 12),

                Align(
                  alignment: Alignment.centerRight,
                  child: GestureDetector(
                    onTap: () => context.push('/forgot-password'),
                    child: const Text('Forgot password?',
                        style: TextStyle(color: Color(0xFFEE1D52),
                            fontSize: 13, fontWeight: FontWeight.w600)),
                  ),
                ),
                const SizedBox(height: 32),

                SizedBox(
                  width: double.infinity,
                  height: 52,
                  child: ElevatedButton(
                    onPressed: _isLoading ? null : _login,
                    style: ElevatedButton.styleFrom(
                      backgroundColor: _isLoading
                          ? Colors.grey[800]
                          : const Color(0xFFEE1D52),
                      disabledBackgroundColor: Colors.grey[800],
                      shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(8)),
                      elevation: 0,
                    ),
                    child: _isLoading
                        ? const SizedBox(
                            width: 22, height: 22,
                            child: CircularProgressIndicator(
                                strokeWidth: 2.5, color: Colors.white))
                        : const Text('Sign In',
                            style: TextStyle(fontSize: 16,
                                fontWeight: FontWeight.w700,
                                color: Colors.white)),
                  ),
                ),

                // Server wake-up hint
                AnimatedSwitcher(
                  duration: const Duration(milliseconds: 300),
                  child: _showWakeHint
                      ? Padding(
                          key: const ValueKey('hint'),
                          padding: const EdgeInsets.only(top: 12),
                          child: Text(
                            'Server is starting up, please wait...',
                            style: TextStyle(color: Colors.grey[500], fontSize: 12),
                          ),
                        )
                      : const SizedBox(key: ValueKey('empty'), height: 12),
                ),

                const SizedBox(height: 20),

                Row(children: [
                  Expanded(child: Divider(color: Colors.grey[800])),
                  Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 16),
                    child: Text('or',
                        style: TextStyle(color: Colors.grey[600], fontSize: 13)),
                  ),
                  Expanded(child: Divider(color: Colors.grey[800])),
                ]),
                const SizedBox(height: 32),

                Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Text("Don't have an account? ",
                        style: TextStyle(color: Colors.grey[500], fontSize: 14)),
                    GestureDetector(
                      onTap: () => context.push('/register'),
                      child: const Text('Sign Up',
                          style: TextStyle(color: Color(0xFFEE1D52),
                              fontSize: 14, fontWeight: FontWeight.w700)),
                    ),
                  ],
                ),
                const SizedBox(height: 32),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
