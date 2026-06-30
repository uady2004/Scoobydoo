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
  final _formKey            = GlobalKey<FormState>();
  final _usernameController = TextEditingController();
  final _emailController    = TextEditingController();
  final _phoneController    = TextEditingController();
  final _passwordController = TextEditingController();
  bool _termsAccepted  = false;
  int  _passwordStrength = 0;

  @override
  void dispose() {
    _usernameController.dispose();
    _emailController.dispose();
    _phoneController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  // ── Password strength ─────────────────────────────────────────────────────
  void _onPasswordChanged(String v) {
    int s = 0;
    if (v.length >= 8)                                           s++;
    if (v.contains(RegExp(r'[A-Z]')))                           s++;
    if (v.contains(RegExp(r'[0-9]')))                           s++;
    if (v.contains(RegExp(r'[!@#\$%^&*(),.?":{}|<>]')))        s++;
    setState(() => _passwordStrength = s);
  }

  Color _strengthColor(int i) {
    if (_passwordStrength == 0 || i >= _passwordStrength) return Colors.grey[800]!;
    return switch (_passwordStrength) {
      1 => const Color(0xFFFF0050),
      2 => const Color(0xFFFF6B00),
      3 => const Color(0xFFFFD700),
      _ => const Color(0xFF00C853),
    };
  }

  String get _strengthLabel => switch (_passwordStrength) {
    0 => '', 1 => 'Weak', 2 => 'Fair', 3 => 'Good', _ => 'Strong',
  };

  // ── Validation ────────────────────────────────────────────────────────────
  String? _validateUsername(String? v) {
    if (v == null || v.trim().isEmpty) return 'Username is required';
    if (v.trim().length < 3) return 'At least 3 characters';
    if (!RegExp(r'^[a-zA-Z0-9_.]+$').hasMatch(v.trim()))
      return 'Only letters, numbers, _ and .';
    return null;
  }

  String? _validateEmail(String? v) {
    if (v == null || v.trim().isEmpty) return 'Email is required';
    if (!RegExp(r'^[^@\s]+@[^@\s]+\.[^@\s]+$').hasMatch(v.trim()))
      return 'Enter a valid email';
    return null;
  }

  String? _validatePassword(String? v) {
    if (v == null || v.isEmpty) return 'Password is required';
    if (v.length < 8) return 'At least 8 characters';
    return null;
  }

  // ── Submit ────────────────────────────────────────────────────────────────
  Future<void> _register() async {
    if (!_formKey.currentState!.validate()) return;
    if (!_termsAccepted) {
      _snack('Please accept the Terms & Privacy Policy.', error: true);
      return;
    }
    await ref.read(authProvider.notifier).register(
      username: _usernameController.text.trim(),
      email:    _emailController.text.trim(),
      password: _passwordController.text,
      phone:    _phoneController.text.trim().isEmpty
                  ? null
                  : _phoneController.text.trim(),
    );
  }

  void _snack(String msg, {bool error = false}) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: Text(msg, style: const TextStyle(color: Colors.white)),
      backgroundColor: error ? const Color(0xFFFF0050) : const Color(0xFF2ECC71),
      behavior: SnackBarBehavior.floating,
    ));
  }

  // ── Build ─────────────────────────────────────────────────────────────────
  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authProvider);
    final isLoading = authState.value is AuthLoading;

    // ── Listen for state changes ──────────────────────────────────────────
    ref.listen(authProvider, (_, next) {
      next.whenData((state) {
        if (state is AuthRegistered) {
          // ✅ Register success → go to login with success message
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
                  controller: _usernameController,
                  label: 'Username',
                  hint: 'your_username',
                  keyboardType: TextInputType.text,
                  prefixIcon: const Icon(Icons.alternate_email),
                  validator: _validateUsername,
                ),
                const SizedBox(height: 16),

                AuthTextField(
                  controller: _emailController,
                  label: 'Email address',
                  hint: 'you@example.com',
                  keyboardType: TextInputType.emailAddress,
                  prefixIcon: const Icon(Icons.email_outlined),
                  validator: _validateEmail,
                ),
                const SizedBox(height: 16),

                AuthTextField(
                  controller: _phoneController,
                  label: 'Phone number (optional)',
                  hint: '+1 234 567 8900',
                  keyboardType: TextInputType.phone,
                  prefixIcon: const Icon(Icons.phone_outlined),
                ),
                const SizedBox(height: 16),

                AuthTextField(
                  controller: _passwordController,
                  label: 'Password',
                  isPassword: true,
                  textInputAction: TextInputAction.done,
                  prefixIcon: const Icon(Icons.lock_outline),
                  validator: _validatePassword,
                  onChanged: _onPasswordChanged,
                ),
                const SizedBox(height: 12),

                // Password strength bar
                Row(
                  children: [
                    ...List.generate(4, (i) => Expanded(
                      child: AnimatedContainer(
                        duration: const Duration(milliseconds: 250),
                        margin: EdgeInsets.only(right: i < 3 ? 4 : 0),
                        height: 4,
                        decoration: BoxDecoration(
                          color: _strengthColor(i),
                          borderRadius: BorderRadius.circular(2),
                        ),
                      ),
                    )),
                    const SizedBox(width: 10),
                    SizedBox(
                      width: 44,
                      child: Text(_strengthLabel,
                          style: TextStyle(
                            color: _strengthColor(_passwordStrength - 1),
                            fontSize: 11, fontWeight: FontWeight.w600,
                          )),
                    ),
                  ],
                ),
                const SizedBox(height: 24),

                // Terms checkbox
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    SizedBox(
                      width: 24, height: 24,
                      child: Checkbox(
                        value: _termsAccepted,
                        onChanged: (v) =>
                            setState(() => _termsAccepted = v ?? false),
                        activeColor: const Color(0xFFFF0050),
                        checkColor: Colors.white,
                        side: BorderSide(color: Colors.grey[600]!, width: 1.5),
                        shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(4)),
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: GestureDetector(
                        onTap: () =>
                            setState(() => _termsAccepted = !_termsAccepted),
                        child: RichText(
                          text: TextSpan(
                            style: TextStyle(color: Colors.grey[400],
                                fontSize: 13, height: 1.5),
                            children: const [
                              TextSpan(text: 'I agree to the '),
                              TextSpan(text: 'Terms of Service',
                                  style: TextStyle(color: Color(0xFFFF0050),
                                      fontWeight: FontWeight.w600)),
                              TextSpan(text: ' and '),
                              TextSpan(text: 'Privacy Policy',
                                  style: TextStyle(color: Color(0xFFFF0050),
                                      fontWeight: FontWeight.w600)),
                            ],
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 32),

                // Create Account button
                SizedBox(
                  width: double.infinity,
                  height: 52,
                  child: DecoratedBox(
                    decoration: BoxDecoration(
                      gradient: isLoading ? null : const LinearGradient(
                        colors: [Color(0xFFFF0050), Color(0xFFEE1D52)],
                      ),
                      color: isLoading ? Colors.grey[800] : null,
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: ElevatedButton(
                      onPressed: isLoading ? null : _register,
                      style: ElevatedButton.styleFrom(
                        backgroundColor: Colors.transparent,
                        shadowColor: Colors.transparent,
                        shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(8)),
                      ),
                      child: isLoading
                          ? const SizedBox(width: 22, height: 22,
                              child: CircularProgressIndicator(
                                  strokeWidth: 2.5, color: Colors.white))
                          : const Text('Create Account',
                              style: TextStyle(fontSize: 16,
                                  fontWeight: FontWeight.w700,
                                  color: Colors.white)),
                    ),
                  ),
                ),
                const SizedBox(height: 24),

                Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Text('Already have an account? ',
                        style: TextStyle(
                            color: Colors.grey[500], fontSize: 14)),
                    GestureDetector(
                      onTap: () => context.go('/login'),
                      child: const Text('Sign In',
                          style: TextStyle(color: Color(0xFFFF0050),
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