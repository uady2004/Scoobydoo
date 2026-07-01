import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../providers/auth_provider.dart';
import '../widgets/auth_text_field.dart';

class RegisterScreen extends ConsumerStatefulWidget {
  const RegisterScreen({super.key});

  @override
  ConsumerState<RegisterScreen> createState() => _RegisterScreenState();
}

class _RegisterScreenState extends ConsumerState<RegisterScreen> {
  final _formKey       = GlobalKey<FormState>();
  final _usernameCtrl  = TextEditingController();
  final _emailCtrl     = TextEditingController();
  final _passwordCtrl  = TextEditingController();
  final _emailFocus    = FocusNode();
  final _passwordFocus = FocusNode();
  bool _loading        = false;
  int  _pwStrength     = 0;

  @override
  void dispose() {
    _usernameCtrl.dispose();
    _emailCtrl.dispose();
    _passwordCtrl.dispose();
    _emailFocus.dispose();
    _passwordFocus.dispose();
    super.dispose();
  }

  void _onPasswordChanged(String v) {
    int s = 0;
    if (v.length >= 8) s++;
    if (v.contains(RegExp(r'[A-Z]'))) s++;
    if (v.contains(RegExp(r'[0-9]'))) s++;
    if (v.contains(RegExp(r'[!@#\$%^&*()\-_=+]'))) s++;
    setState(() => _pwStrength = s);
  }

  Color get _strengthColor => switch (_pwStrength) {
        1 => Colors.red[400]!,
        2 => Colors.orange[400]!,
        3 => Colors.yellow[600]!,
        4 => Colors.green[400]!,
        _ => Colors.transparent,
      };

  String get _strengthLabel => switch (_pwStrength) {
        1 => 'Weak',
        2 => 'Fair',
        3 => 'Good',
        4 => 'Strong',
        _ => '',
      };

  Future<void> _createAccount() async {
    FocusScope.of(context).unfocus();
    if (!_formKey.currentState!.validate()) return;

    setState(() => _loading = true);
    try {
      await ref.read(authProvider.notifier).register(
            username: _usernameCtrl.text.trim(),
            email: _emailCtrl.text.trim(),
            password: _passwordCtrl.text,
          );
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  void _showError(String msg) {
    if (!mounted) return;
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(
        SnackBar(
          content: Text(msg, style: const TextStyle(color: Colors.white)),
          backgroundColor: const Color(0xFFEE1D52),
          behavior: SnackBarBehavior.floating,
          margin: const EdgeInsets.fromLTRB(16, 0, 16, 24),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
          duration: const Duration(seconds: 4),
        ),
      );
  }

  @override
  Widget build(BuildContext context) {
    // React to auth state changes
    ref.listen<AsyncValue<AuthState>>(authProvider, (_, next) {
      next.whenData((state) {
        if (state is AuthAuthenticated) {
          // Backend returns tokens on register → auto logged in → go to home
          context.go('/home');
        } else if (state is AuthError) {
          _showError(state.message);
          ref.read(authProvider.notifier).clearError();
        }
      });
    });

    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        elevation: 0,
        scrolledUnderElevation: 0,
        leading: IconButton(
          icon: const Icon(Icons.arrow_back_ios_new_rounded,
              color: Colors.white, size: 20),
          onPressed: _loading ? null : () => context.pop(),
        ),
        title: const Text(
          'Create account',
          style: TextStyle(
            color: Colors.white,
            fontSize: 18,
            fontWeight: FontWeight.w700,
          ),
        ),
        centerTitle: true,
      ),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.symmetric(horizontal: 28),
          child: Form(
            key: _formKey,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SizedBox(height: 20),

                // ── Username ──────────────────────────────────────────────
                AuthTextField(
                  controller: _usernameCtrl,
                  label: 'Username',
                  hint: 'your_username',
                  prefixIcon: const Icon(Icons.alternate_email_rounded),
                  textInputAction: TextInputAction.next,
                  enabled: !_loading,
                  onSubmitted: (_) => _emailFocus.requestFocus(),
                  validator: (v) {
                    if (v == null || v.trim().isEmpty) {
                      return 'Username is required';
                    }
                    final trimmed = v.trim();
                    if (trimmed.length < 3) {
                      return 'Username must be at least 3 characters';
                    }
                    if (trimmed.length > 30) {
                      return 'Username must be 30 characters or less';
                    }
                    if (!RegExp(r'^[a-zA-Z0-9_]+$').hasMatch(trimmed)) {
                      return 'Only letters, numbers and underscores allowed';
                    }
                    return null;
                  },
                ),
                const SizedBox(height: 14),

                // ── Email ─────────────────────────────────────────────────
                AuthTextField(
                  controller: _emailCtrl,
                  label: 'Email',
                  hint: 'you@example.com',
                  keyboardType: TextInputType.emailAddress,
                  textInputAction: TextInputAction.next,
                  prefixIcon: const Icon(Icons.email_outlined),
                  focusNode: _emailFocus,
                  enabled: !_loading,
                  onSubmitted: (_) => _passwordFocus.requestFocus(),
                  validator: (v) {
                    if (v == null || v.trim().isEmpty) {
                      return 'Email is required';
                    }
                    if (!RegExp(r'^[^@]+@[^@]+\.[^@]+').hasMatch(v.trim())) {
                      return 'Enter a valid email address';
                    }
                    return null;
                  },
                ),
                const SizedBox(height: 14),

                // ── Password ──────────────────────────────────────────────
                AuthTextField(
                  controller: _passwordCtrl,
                  label: 'Password',
                  isPassword: true,
                  focusNode: _passwordFocus,
                  textInputAction: TextInputAction.done,
                  prefixIcon: const Icon(Icons.lock_outline_rounded),
                  enabled: !_loading,
                  onChanged: _onPasswordChanged,
                  onSubmitted: (_) => _createAccount(),
                  validator: (v) {
                    if (v == null || v.isEmpty) return 'Password is required';
                    if (v.length < 8) {
                      return 'Password must be at least 8 characters';
                    }
                    return null;
                  },
                ),
                const SizedBox(height: 10),

                // ── Password strength ─────────────────────────────────────
                if (_pwStrength > 0) ...[
                  Row(
                    children: [
                      ...List.generate(
                        4,
                        (i) => Expanded(
                          child: AnimatedContainer(
                            duration: const Duration(milliseconds: 250),
                            margin: EdgeInsets.only(right: i < 3 ? 4 : 0),
                            height: 3,
                            decoration: BoxDecoration(
                              color: i < _pwStrength
                                  ? _strengthColor
                                  : Colors.grey[800],
                              borderRadius: BorderRadius.circular(2),
                            ),
                          ),
                        ),
                      ),
                      const SizedBox(width: 10),
                      AnimatedDefaultTextStyle(
                        duration: const Duration(milliseconds: 200),
                        style: TextStyle(
                          color: _strengthColor,
                          fontSize: 11,
                          fontWeight: FontWeight.w600,
                        ),
                        child: Text(_strengthLabel),
                      ),
                    ],
                  ),
                  const SizedBox(height: 28),
                ] else
                  const SizedBox(height: 28),

                // ── Create Account button ─────────────────────────────────
                SizedBox(
                  width: double.infinity,
                  height: 52,
                  child: ElevatedButton(
                    onPressed: _loading ? null : _createAccount,
                    style: ElevatedButton.styleFrom(
                      backgroundColor: const Color(0xFFEE1D52),
                      disabledBackgroundColor: Colors.grey[850],
                      foregroundColor: Colors.white,
                      disabledForegroundColor: Colors.white54,
                      elevation: 0,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(8),
                      ),
                    ),
                    child: _loading
                        ? const SizedBox(
                            width: 22,
                            height: 22,
                            child: CircularProgressIndicator(
                              strokeWidth: 2.5,
                              color: Colors.white,
                            ),
                          )
                        : const Text(
                            'Create Account',
                            style: TextStyle(
                              fontSize: 16,
                              fontWeight: FontWeight.w700,
                              letterSpacing: 0.3,
                            ),
                          ),
                  ),
                ),
                const SizedBox(height: 28),

                // ── Sign In link ──────────────────────────────────────────
                Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Text(
                      'Already have an account?  ',
                      style: TextStyle(color: Colors.grey[500], fontSize: 14),
                    ),
                    GestureDetector(
                      onTap: _loading ? null : () => context.go('/login'),
                      child: const Text(
                        'Sign In',
                        style: TextStyle(
                          color: Color(0xFFEE1D52),
                          fontSize: 14,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
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
