import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../providers/auth_provider.dart';

class SplashScreen extends ConsumerStatefulWidget {
  const SplashScreen({super.key});

  @override
  ConsumerState<SplashScreen> createState() => _SplashScreenState();
}

class _SplashScreenState extends ConsumerState<SplashScreen>
    with SingleTickerProviderStateMixin {
  late final AnimationController _controller;
  late final Animation<double> _scaleAnim;
  late final Animation<double> _fadeAnim;
  late final Animation<double> _subtitleFadeAnim;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 800),
    );

    _scaleAnim = Tween<double>(begin: 0.8, end: 1.0).animate(
      CurvedAnimation(parent: _controller, curve: Curves.easeOutCubic),
    );

    _fadeAnim = Tween<double>(begin: 0.0, end: 1.0).animate(
      CurvedAnimation(
        parent: _controller,
        curve: const Interval(0.0, 0.7, curve: Curves.easeIn),
      ),
    );

    _subtitleFadeAnim = Tween<double>(begin: 0.0, end: 1.0).animate(
      CurvedAnimation(
        parent: _controller,
        curve: const Interval(0.5, 1.0, curve: Curves.easeIn),
      ),
    );

    _controller.forward();
    Future.delayed(const Duration(milliseconds: 2000), _navigate);
  }

  Future<void> _navigate() async {
    if (!mounted) return;
    final authState = ref.read(authProvider);
    final state = authState.when(
      data: (s) => s,
      loading: () => null,
      error: (_, __) => const AuthUnauthenticated(),
    );

    if (state == null) {
      await Future.delayed(const Duration(milliseconds: 400));
      if (mounted) _navigate();
      return;
    }

    if (!mounted) return;
    if (state is AuthAuthenticated) {
      context.go('/home');
    } else {
      context.go('/login');
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      body: Center(
        child: AnimatedBuilder(
          animation: _controller,
          builder: (context, child) {
            return Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                FadeTransition(
                  opacity: _fadeAnim,
                  child: ScaleTransition(
                    scale: _scaleAnim,
                    child: _TikTokLogo(),
                  ),
                ),
                const SizedBox(height: 14),
                FadeTransition(
                  opacity: _subtitleFadeAnim,
                  child: const Text(
                    'by Clone Studio',
                    style: TextStyle(
                      color: Color(0xFF6B6B6B),
                      fontSize: 13,
                      letterSpacing: 0.5,
                      fontWeight: FontWeight.w400,
                    ),
                  ),
                ),
              ],
            );
          },
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// TikTok glitch-layer logo with red/teal gradient effect
// ---------------------------------------------------------------------------

class _TikTokLogo extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    const text = 'TikTok';
    const fontSize = 52.0;
    const fontWeight = FontWeight.w900;
    const letterSpacing = -1.0;

    return Stack(
      children: [
        // Teal shadow layer (offset left)
        Transform.translate(
          offset: const Offset(-2.5, 0),
          child: Text(
            text,
            style: TextStyle(
              fontSize: fontSize,
              fontWeight: fontWeight,
              letterSpacing: letterSpacing,
              foreground: Paint()
                ..color = const Color(0xFF69C9D0).withValues(alpha: 0.75),
            ),
          ),
        ),
        // Red shadow layer (offset right)
        Transform.translate(
          offset: const Offset(2.5, 0),
          child: Text(
            text,
            style: TextStyle(
              fontSize: fontSize,
              fontWeight: fontWeight,
              letterSpacing: letterSpacing,
              foreground: Paint()
                ..color = const Color(0xFFEE1D52).withValues(alpha: 0.75),
            ),
          ),
        ),
        // Primary white layer
        const Text(
          text,
          style: TextStyle(
            fontSize: fontSize,
            fontWeight: fontWeight,
            letterSpacing: letterSpacing,
            color: Colors.white,
          ),
        ),
      ],
    );
  }
}
