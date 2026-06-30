import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:sign_in_with_apple/sign_in_with_apple.dart';
import '../providers/auth_provider.dart';
import '../widgets/auth_text_field.dart';
import '../widgets/social_login_button.dart';

class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});

  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen>
    with SingleTickerProviderStateMixin {
  late final TabController _tabController;
  final _emailController    = TextEditingController();
  final _phoneController    = TextEditingController();
  final _passwordController = TextEditingController();
  final _formKey            = GlobalKey<FormState>();
  bool _socialLoading = false;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
  }

  @override
  void dispose() {
    _tabController.dispose();
    _emailController.dispose();
    _phoneController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  // ── Actions ───────────────────────────────────────────────────────────────
  Future<void> _signIn() async {
    if (!_formKey.currentState!.validate()) return;
    // tab 0 = phone, tab 1 = email
    final isEmail = _tabController.index == 1;
    await ref.read(authProvider.notifier).login(
      email:    isEmail
                  ? _emailController.text.trim()
                  : _phoneController.text.trim(),
      password: _passwordController.text,
    );
  }

  Future<void> _googleSignIn() async {
    setState(() => _socialLoading = true);
    await ref.read(authProvider.notifier).googleSignIn();
    if (mounted) setState(() => _socialLoading = false);
  }

  Future<void> _appleSignIn() async {
    try {
      setState(() => _socialLoading = true);
      final cred = await SignInWithApple.getAppleIDCredential(scopes: [
        AppleIDAuthorizationScopes.email,
        AppleIDAuthorizationScopes.fullName,
      ]);
      if (!mounted) return;
      await ref.read(authProvider.notifier).appleSignIn(
        identityToken: cred.identityToken ?? '',
      );
    } catch (_) {
    } finally {
      if (mounted) setState(() => _socialLoading = false);
    }
  }

  void _snack(String msg, {bool error = true}) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: Text(msg, style: const TextStyle(color: Colors.white)),
      backgroundColor: error ? const Color(0xFFFF0050) : const Color(0xFF2ECC71),
      behavior: SnackBarBehavior.floating,
    ));
  }

  // ── Validation ────────────────────────────────────────────────────────────
  String? _validateEmail(String? v) {
    if (v == null || v.trim().isEmpty) return 'Email is required';
    if (!RegExp(r'^[^@\s]+@[^@\s]+\.[^@\s]+$').hasMatch(v.trim()))
      return 'Enter a valid email';
    return null;
  }

  String? _validatePassword(String? v) {
    if (v == null || v.isEmpty) return 'Password is required';
    if (v.length < 6) return 'At least 6 characters';
    return null;
  }

  // ── Build ─────────────────────────────────────────────────────────────────
  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authProvider);
    final isLoading = authState.value is AuthLoading;

    // ── Listen for state changes ──────────────────────────────────────────
    ref.listen(authProvider, (_, next) {
      next.whenData((state) {
        if (state is AuthAuthenticated) {
          // ✅ Login success → go to home
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
                const SizedBox(height: 48),

                // Logo / title
                const Text('TikTok',
                    style: TextStyle(color: Colors.white, fontSize: 36,
                        fontWeight: FontWeight.w900,
                        letterSpacing: -1)),
                const SizedBox(height: 8),
                Text('Sign in to your account',
                    style: TextStyle(color: Colors.grey[400], fontSize: 14)),
                const SizedBox(height: 40),

                // Phone / Email tab bar
                Container(
                  height: 40,
                  decoration: BoxDecoration(
                    color: Colors.white10,
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: TabBar(
                    controller: _tabController,
                    indicator: BoxDecoration(
                      color: const Color(0xFFFF0050),
                      borderRadius: BorderRadius.circular(8),
                    ),
                    indicatorSize: TabBarIndicatorSize.tab,
                    dividerColor: Colors.transparent,
                    labelColor: Colors.white,
                    unselectedLabelColor: Colors.white54,
                    labelStyle: const TextStyle(
                        fontSize: 13, fontWeight: FontWeight.w600),
                    tabs: const [
                      Tab(text: 'Phone'),
                      Tab(text: 'Email'),
                    ],
                  ),
                ),
                const SizedBox(height: 24),

                // Tab content
                SizedBox(
                  height: 64,
                  child: TabBarView(
                    controller: _tabController,
                    children: [
                      // Phone tab
                      AuthTextField(
                        controller: _phoneController,
                        label: 'Phone number',
                        hint: '+1 234 567 8900',
                        keyboardType: TextInputType.phone,
                        prefixIcon: const Icon(Icons.phone_outlined),
                        validator: (v) => v == null || v.trim().isEmpty
                            ? 'Phone is required'
                            : null,
                      ),
                      // Email tab
                      AuthTextField(
                        controller: _emailController,
                        label: 'Email address',
                        hint: 'you@example.com',
                        keyboardType: TextInputType.emailAddress,
                        prefixIcon: const Icon(Icons.email_outlined),
                        validator: _validateEmail,
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 16),

                // Password
                AuthTextField(
                  controller: _passwordController,
                  label: 'Password',
                  isPassword: true,
                  textInputAction: TextInputAction.done,
                  prefixIcon: const Icon(Icons.lock_outline),
                  validator: _validatePassword,
                  onSubmitted: (_) => isLoading ? null : _signIn(),
                ),
                const SizedBox(height: 12),

                // Forgot password
                Align(
                  alignment: Alignment.centerRight,
                  child: GestureDetector(
                    onTap: () => context.push('/forgot-password'),
                    child: const Text('Forgot password?',
                        style: TextStyle(color: Color(0xFFFF0050),
                            fontSize: 13, fontWeight: FontWeight.w600)),
                  ),
                ),
                const SizedBox(height: 32),

                // Sign In button
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
                      onPressed: isLoading ? null : _signIn,
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
                          : const Text('Sign In',
                              style: TextStyle(fontSize: 16,
                                  fontWeight: FontWeight.w700,
                                  color: Colors.white)),
                    ),
                  ),
                ),
                const SizedBox(height: 28),

                // Divider
                Row(children: [
                  Expanded(child: Divider(color: Colors.grey[800])),
                  Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 12),
                    child: Text('or continue with',
                        style: TextStyle(color: Colors.grey[600], fontSize: 12)),
                  ),
                  Expanded(child: Divider(color: Colors.grey[800])),
                ]),
                const SizedBox(height: 24),

                // Social login buttons
                SocialLoginButton(
                  provider: SocialProvider.google,
                  onPressed: _googleSignIn,
                  isLoading: _socialLoading,
                ),
                const SizedBox(height: 12),
                SocialLoginButton(
                  provider: SocialProvider.apple,
                  onPressed: _appleSignIn,
                  isLoading: _socialLoading,
                ),
                const SizedBox(height: 40),

                // Register link
                Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Text("Don't have an account? ",
                        style: TextStyle(
                            color: Colors.grey[500], fontSize: 14)),
                    GestureDetector(
                      onTap: () => context.push('/register'),
                      child: const Text('Sign Up',
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