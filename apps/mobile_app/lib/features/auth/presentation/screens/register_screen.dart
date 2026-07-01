import 'dart:async';

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
  final _formKey      = GlobalKey<FormState>();
  final _usernameCtrl = TextEditingController();
  final _emailCtrl    = TextEditingController();
  final _phoneCtrl    = TextEditingController();
  final _passwordCtrl = TextEditingController();
  bool   _accepted    = false;
  int    _strength    = 0;
  bool   _isLoading   = false;
  bool   _showWakeHint = false;
  Timer? _wakeTimer;

  @override
  void dispose() {
    _wakeTimer?.cancel();
    _usernameCtrl.dispose();
    _emailCtrl.dispose();
    _phoneCtrl.dispose();
    _passwordCtrl.dispose();
    super.dispose();
  }

  void _onPassword(String v) {
    int s = 0;
    if (v.length >= 8)                     s++;
    if (v.contains(RegExp(r'[A-Z]')))      s++;
    if (v.contains(RegExp(r'[0-9]')))      s++;
    if (v.contains(RegExp(r'[!@#\$%^&*]'))) s++;
    setState(() => _strength = s);
  }

  Color _bar(int i) {
    if (_strength == 0 || i >= _strength) return Colors.grey[800]!;
    return [Colors.red, Colors.orange, Colors.yellow, Colors.green][_strength - 1];
  }

  String get _label => ['', 'Weak', 'Fair', 'Good', 'Strong'][_strength];

  Future<void> _register() async {
    if (!_formKey.currentState!.validate()) return;
    if (!_accepted) {
      _snack('Please accept Terms & Privacy Policy', error: true);
      return;
    }

    setState(() { _isLoading = true; _showWakeHint = false; });

    _wakeTimer = Timer(const Duration(seconds: 4), () {
      if (mounted && _isLoading) setState(() => _showWakeHint = true);
    });

    try {
      await ref.read(authProvider.notifier).register(
        username: _usernameCtrl.text.trim(),
        email:    _emailCtrl.text.trim(),
        password: _passwordCtrl.text,
        phone:    _phoneCtrl.text.trim().isEmpty ? null : _phoneCtrl.text.trim(),
      );
    } finally {
      _wakeTimer?.cancel();
      if (mounted) setState(() { _isLoading = false; _showWakeHint = false; });
    }
  }

  void _snack(String msg, {bool error = false}) {
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
        if (state is AuthRegistered) {
          _snack('Account created! Please sign in.');
          context.go('/login');
        } else if (state is AuthError) {
          _snack(state.message, error: true);
          ref.read(authProvider.notifier).clearError();
        }
      });
    });

    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        elevation: 0,
        leading: IconButton(
          icon: const Icon(Icons.arrow_back_ios, color: Colors.white, size: 20),
          onPressed: () => context.pop(),
        ),
        title: const Text('Create account',
            style: TextStyle(color: Colors.white, fontSize: 17,
                fontWeight: FontWeight.w600)),
        centerTitle: true,
      ),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.symmetric(horizontal: 24),
          child: Form(
            key: _formKey,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SizedBox(height: 24),

                AuthTextField(
                  controller: _usernameCtrl,
                  label: 'Username',
                  hint: 'your_username',
                  keyboardType: TextInputType.text,
                  prefixIcon: const Icon(Icons.alternate_email),
                  validator: (v) {
                    if (v == null || v.trim().isEmpty) return 'Username required';
                    if (v.trim().length < 3) return 'At least 3 characters';
                    return null;
                  },
                ),
                const SizedBox(height: 16),

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
                  controller: _phoneCtrl,
                  label: 'Phone number (optional)',
                  hint: '+1 234 567 8900',
                  keyboardType: TextInputType.phone,
                  prefixIcon: const Icon(Icons.phone_outlined),
                ),
                const SizedBox(height: 16),

                AuthTextField(
                  controller: _passwordCtrl,
                  label: 'Password',
                  isPassword: true,
                  prefixIcon: const Icon(Icons.lock_outline),
                  onChanged: _onPassword,
                  validator: (v) {
                    if (v == null || v.isEmpty) return 'Password required';
                    if (v.length < 8) return 'At least 8 characters';
                    return null;
                  },
                ),
                const SizedBox(height: 10),

                if (_strength > 0) ...[
                  Row(
                    children: [
                      ...List.generate(4, (i) => Expanded(
                        child: AnimatedContainer(
                          duration: const Duration(milliseconds: 200),
                          margin: EdgeInsets.only(right: i < 3 ? 4 : 0),
                          height: 4,
                          decoration: BoxDecoration(
                            color: _bar(i),
                            borderRadius: BorderRadius.circular(2),
                          ),
                        ),
                      )),
                      const SizedBox(width: 8),
                      Text(_label,
                          style: TextStyle(
                              color: _bar(_strength - 1),
                              fontSize: 11,
                              fontWeight: FontWeight.w600)),
                    ],
                  ),
                  const SizedBox(height: 16),
                ] else
                  const SizedBox(height: 16),

                Row(
                  crossAxisAlignment: CrossAxisAlignment.center,
                  children: [
                    Checkbox(
                      value: _accepted,
                      onChanged: (v) => setState(() => _accepted = v ?? false),
                      activeColor: const Color(0xFFEE1D52),
                      checkColor: Colors.white,
                      side: BorderSide(color: Colors.grey[600]!),
                      shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(4)),
                    ),
                    Expanded(
                      child: GestureDetector(
                        onTap: () => setState(() => _accepted = !_accepted),
                        child: RichText(
                          text: TextSpan(
                            style: TextStyle(color: Colors.grey[400], fontSize: 13),
                            children: const [
                              TextSpan(text: 'I agree to the '),
                              TextSpan(text: 'Terms of Service',
                                  style: TextStyle(color: Color(0xFFEE1D52),
                                      fontWeight: FontWeight.w600)),
                              TextSpan(text: ' and '),
                              TextSpan(text: 'Privacy Policy',
                                  style: TextStyle(color: Color(0xFFEE1D52),
                                      fontWeight: FontWeight.w600)),
                            ],
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 28),

                SizedBox(
                  width: double.infinity,
                  height: 52,
                  child: ElevatedButton(
                    onPressed: _isLoading ? null : _register,
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
                        : const Text('Create Account',
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
                          child: Center(
                            child: Text(
                              'Server is starting up, please wait...',
                              style: TextStyle(color: Colors.grey[500], fontSize: 12),
                            ),
                          ),
                        )
                      : const SizedBox(key: ValueKey('empty'), height: 12),
                ),

                const SizedBox(height: 8),

                Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Text('Already have an account? ',
                        style: TextStyle(color: Colors.grey[500], fontSize: 14)),
                    GestureDetector(
                      onTap: () => context.go('/login'),
                      child: const Text('Sign In',
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
