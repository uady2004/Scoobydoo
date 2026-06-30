import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

class OnboardingScreen extends StatefulWidget {
  const OnboardingScreen({super.key});

  @override
  State<OnboardingScreen> createState() => _OnboardingScreenState();
}

class _OnboardingScreenState extends State<OnboardingScreen> {
  final PageController _pageController = PageController();
  int _currentPage = 0;

  static const _pages = [
    _OnboardingPage(
      gradient: [Color(0xFF0A0E1A), Color(0xFF0D1B3E), Color(0xFF0F3460)],
      icon: Icons.play_circle_fill_rounded,
      iconColor: Color(0xFF69C9D0),
      title: 'Discover Trending Videos',
      subtitle:
          'Watch millions of videos from creators around the world — '
          'curated just for you, updated every hour.',
    ),
    _OnboardingPage(
      gradient: [Color(0xFF1A0810), Color(0xFF3D1020), Color(0xFF6B1A2A)],
      icon: Icons.upload_rounded,
      iconColor: Color(0xFFEE1D52),
      title: 'Create & Share',
      subtitle:
          'Express yourself with short videos, duets, and stitches. '
          'Your story deserves an audience.',
    ),
    _OnboardingPage(
      gradient: [Color(0xFF0D0A1F), Color(0xFF1E1060), Color(0xFF3D2A8A)],
      icon: Icons.people_alt_rounded,
      iconColor: Color(0xFFBB86FC),
      title: 'Connect & Grow',
      subtitle:
          'Build your community and become a creator. '
          'Follow, react, and grow alongside people who get you.',
    ),
  ];

  void _onPageChanged(int index) => setState(() => _currentPage = index);

  void _next() {
    if (_currentPage < _pages.length - 1) {
      _pageController.nextPage(
        duration: const Duration(milliseconds: 350),
        curve: Curves.easeInOut,
      );
    } else {
      context.go('/login');
    }
  }

  void _skip() => context.go('/login');

  @override
  void dispose() {
    _pageController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final isLast = _currentPage == _pages.length - 1;

    return Scaffold(
      backgroundColor: Colors.black,
      body: Stack(
        children: [
          // Pages
          PageView.builder(
            controller: _pageController,
            onPageChanged: _onPageChanged,
            itemCount: _pages.length,
            itemBuilder: (context, index) =>
                _OnboardingPageView(page: _pages[index]),
          ),

          // Skip button (top-right, hidden on last page)
          if (!isLast)
            Positioned(
              top: MediaQuery.of(context).padding.top + 12,
              right: 20,
              child: TextButton(
                onPressed: _skip,
                child: const Text(
                  'Skip',
                  style: TextStyle(
                    color: Colors.white70,
                    fontSize: 15,
                    fontWeight: FontWeight.w500,
                  ),
                ),
              ),
            ),

          // Bottom controls
          Positioned(
            left: 0,
            right: 0,
            bottom: MediaQuery.of(context).padding.bottom + 40,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                // Page indicators — 3 dots, active = red, wider pill
                Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: List.generate(
                    _pages.length,
                    (i) => AnimatedContainer(
                      duration: const Duration(milliseconds: 300),
                      curve: Curves.easeInOut,
                      margin: const EdgeInsets.symmetric(horizontal: 4),
                      width: _currentPage == i ? 24 : 8,
                      height: 8,
                      decoration: BoxDecoration(
                        color: _currentPage == i
                            ? const Color(0xFFFF0050)
                            : Colors.white30,
                        borderRadius: BorderRadius.circular(4),
                      ),
                    ),
                  ),
                ),
                const SizedBox(height: 36),

                // CTA button
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 32),
                  child: SizedBox(
                    width: double.infinity,
                    height: 54,
                    child: DecoratedBox(
                      decoration: BoxDecoration(
                        gradient: const LinearGradient(
                          colors: [Color(0xFFEE1D52), Color(0xFFFF0050)],
                        ),
                        borderRadius: BorderRadius.circular(12),
                        boxShadow: [
                          BoxShadow(
                            color: const Color(0xFFEE1D52).withValues(alpha: 0.35),
                            blurRadius: 20,
                            offset: const Offset(0, 8),
                          ),
                        ],
                      ),
                      child: ElevatedButton(
                        onPressed: _next,
                        style: ElevatedButton.styleFrom(
                          backgroundColor: Colors.transparent,
                          shadowColor: Colors.transparent,
                          minimumSize: Size.zero,
                          shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(12),
                          ),
                        ),
                        child: Text(
                          isLast ? 'Get Started' : 'Next',
                          style: const TextStyle(
                            fontSize: 16,
                            fontWeight: FontWeight.w700,
                            color: Colors.white,
                            letterSpacing: 0.3,
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Data model
// ---------------------------------------------------------------------------

class _OnboardingPage {
  final List<Color> gradient;
  final IconData icon;
  final Color iconColor;
  final String title;
  final String subtitle;

  const _OnboardingPage({
    required this.gradient,
    required this.icon,
    required this.iconColor,
    required this.title,
    required this.subtitle,
  });
}

// ---------------------------------------------------------------------------
// Page widget
// ---------------------------------------------------------------------------

class _OnboardingPageView extends StatelessWidget {
  final _OnboardingPage page;

  const _OnboardingPageView({required this.page});

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topCenter,
          end: Alignment.bottomCenter,
          colors: page.gradient,
        ),
      ),
      child: SafeArea(
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 32),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              const Spacer(flex: 2),
              // Icon with ambient glow
              Container(
                width: 152,
                height: 152,
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  color: Colors.white.withValues(alpha: 0.06),
                  border: Border.all(
                    color: page.iconColor.withValues(alpha: 0.2),
                    width: 1.5,
                  ),
                  boxShadow: [
                    BoxShadow(
                      color: page.iconColor.withValues(alpha: 0.4),
                      blurRadius: 70,
                      spreadRadius: 8,
                    ),
                  ],
                ),
                child: Icon(page.icon, size: 72, color: page.iconColor),
              ),
              const Spacer(flex: 1),
              Text(
                page.title,
                textAlign: TextAlign.center,
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 30,
                  fontWeight: FontWeight.w800,
                  letterSpacing: -0.5,
                  height: 1.15,
                ),
              ),
              const SizedBox(height: 20),
              Text(
                page.subtitle,
                textAlign: TextAlign.center,
                style: TextStyle(
                  color: Colors.white.withValues(alpha: 0.7),
                  fontSize: 16,
                  height: 1.6,
                  fontWeight: FontWeight.w400,
                ),
              ),
              // Reserve space for the bottom controls overlay
              const SizedBox(height: 180),
            ],
          ),
        ),
      ),
    );
  }
}
