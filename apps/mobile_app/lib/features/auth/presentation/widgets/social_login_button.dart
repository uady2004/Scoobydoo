import 'package:flutter/material.dart';

enum SocialProvider { google, apple }

class SocialLoginButton extends StatelessWidget {
  final SocialProvider provider;
  final VoidCallback onPressed;
  final bool isLoading;

  const SocialLoginButton({
    super.key,
    required this.provider,
    required this.onPressed,
    this.isLoading = false,
  });

  @override
  Widget build(BuildContext context) {
    return switch (provider) {
      SocialProvider.google => _GoogleButton(
          onPressed: isLoading ? null : onPressed,
          isLoading: isLoading,
        ),
      SocialProvider.apple => _AppleButton(
          onPressed: isLoading ? null : onPressed,
          isLoading: isLoading,
        ),
    };
  }
}

// ---------------------------------------------------------------------------
// Google button
// ---------------------------------------------------------------------------

class _GoogleButton extends StatelessWidget {
  final VoidCallback? onPressed;
  final bool isLoading;

  const _GoogleButton({this.onPressed, required this.isLoading});

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: double.infinity,
      height: 52,
      child: OutlinedButton(
        onPressed: onPressed,
        style: OutlinedButton.styleFrom(
          backgroundColor: Colors.white,
          foregroundColor: const Color(0xFF1A1A1A),
          side: const BorderSide(color: Color(0xFFDDDDDD), width: 1),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(8),
          ),
        ),
        child: isLoading
            ? const SizedBox(
                width: 20,
                height: 20,
                child: CircularProgressIndicator(
                  strokeWidth: 2,
                  color: Color(0xFF1A1A1A),
                ),
              )
            : Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  _GoogleLogoSvg(),
                  const SizedBox(width: 12),
                  const Text(
                    'Continue with Google',
                    style: TextStyle(
                      fontSize: 15,
                      fontWeight: FontWeight.w500,
                      color: Color(0xFF1A1A1A),
                    ),
                  ),
                ],
              ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Apple button
// ---------------------------------------------------------------------------

class _AppleButton extends StatelessWidget {
  final VoidCallback? onPressed;
  final bool isLoading;

  const _AppleButton({this.onPressed, required this.isLoading});

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: double.infinity,
      height: 52,
      child: ElevatedButton(
        onPressed: onPressed,
        style: ElevatedButton.styleFrom(
          backgroundColor: Colors.white,
          foregroundColor: Colors.black,
          elevation: 0,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(8),
          ),
        ),
        child: isLoading
            ? const SizedBox(
                width: 20,
                height: 20,
                child: CircularProgressIndicator(
                  strokeWidth: 2,
                  color: Colors.black,
                ),
              )
            : Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  _AppleLogoSvg(),
                  const SizedBox(width: 10),
                  const Text(
                    'Continue with Apple',
                    style: TextStyle(
                      fontSize: 15,
                      fontWeight: FontWeight.w500,
                      color: Colors.black,
                    ),
                  ),
                ],
              ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Inline SVG logos (no external asset needed)
// ---------------------------------------------------------------------------

class _GoogleLogoSvg extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    // Google "G" rendered with a CustomPaint to avoid asset/CDN dependency.
    return CustomPaint(
      size: const Size(20, 20),
      painter: _GoogleLogoPainter(),
    );
  }
}

class _GoogleLogoPainter extends CustomPainter {
  @override
  void paint(Canvas canvas, Size size) {
    final cx = size.width / 2;
    final cy = size.height / 2;
    final r = size.width / 2;

    // Draw colored arc segments.
    final segments = [
      (Colors.red, 0.0, 0.5),
      (Colors.yellow, 0.5, 0.33),
      (Colors.green, 0.83, 0.17),
      (Colors.blue, 0.0, -0.42),
    ];

    for (final (color, startFraction, sweepFraction) in segments) {
      final paint = Paint()
        ..color = color
        ..style = PaintingStyle.stroke
        ..strokeWidth = 3.5
        ..strokeCap = StrokeCap.butt;

      canvas.drawArc(
        Rect.fromCircle(center: Offset(cx, cy), radius: r - 1.75),
        startFraction * 2 * 3.14159,
        sweepFraction * 2 * 3.14159,
        false,
        paint,
      );
    }

    // White cutout + horizontal bar for the "G" shape.
    final whitePaint = Paint()
      ..color = Colors.white
      ..style = PaintingStyle.fill;
    canvas.drawRect(
      Rect.fromLTWH(cx, cy - 2, r - 1.5, 4),
      whitePaint,
    );
    canvas.drawRect(
      Rect.fromLTWH(cx, cy - 2.5, r + 0.5, 5),
      Paint()
        ..color = const Color(0xFF4285F4)
        ..style = PaintingStyle.fill,
    );
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}

class _AppleLogoSvg extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return CustomPaint(
      size: const Size(18, 22),
      painter: _AppleLogoPainter(),
    );
  }
}

class _AppleLogoPainter extends CustomPainter {
  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = Colors.black
      ..style = PaintingStyle.fill;

    final path = Path();
    final w = size.width;
    final h = size.height;

    // Simplified Apple logo path (proportional).
    path.moveTo(w * 0.5, h * 0.13);
    path.cubicTo(w * 0.62, h * 0.01, w * 0.85, h * 0.05, w * 0.85, h * 0.05);
    path.cubicTo(w * 0.85, h * 0.05, w * 0.72, h * 0.12, w * 0.72, h * 0.26);
    path.cubicTo(w * 0.72, h * 0.43, w * 0.88, h * 0.49, w * 0.88, h * 0.49);
    path.cubicTo(w * 0.80, h * 0.66, w * 0.74, h * 0.72, w * 0.66, h * 0.72);
    path.cubicTo(w * 0.58, h * 0.72, w * 0.54, h * 0.68, w * 0.46, h * 0.68);
    path.cubicTo(w * 0.38, h * 0.68, w * 0.34, h * 0.72, w * 0.26, h * 0.72);
    path.cubicTo(w * 0.18, h * 0.72, w * 0.13, h * 0.66, w * 0.06, h * 0.49);
    path.cubicTo(w * -0.03, h * 0.31, w * 0.02, h * 0.05, w * 0.17, h * 0.05);
    path.cubicTo(w * 0.26, h * 0.05, w * 0.31, h * 0.10, w * 0.38, h * 0.10);
    path.cubicTo(w * 0.44, h * 0.10, w * 0.48, h * 0.06, w * 0.5, h * 0.13);
    path.close();

    // Leaf.
    path.moveTo(w * 0.62, h * 0.0);
    path.cubicTo(w * 0.62, h * 0.0, w * 0.65, h * 0.10, w * 0.57, h * 0.17);
    path.cubicTo(w * 0.49, h * 0.24, w * 0.40, h * 0.19, w * 0.40, h * 0.19);
    path.cubicTo(w * 0.40, h * 0.19, w * 0.40, h * 0.09, w * 0.48, h * 0.04);
    path.cubicTo(w * 0.56, h * -0.01, w * 0.62, h * 0.0, w * 0.62, h * 0.0);
    path.close();

    canvas.drawPath(path, paint);
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}
