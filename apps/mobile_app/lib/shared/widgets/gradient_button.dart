import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';

/// A full-width 50px button with the brand red-to-pink gradient.
///
/// Shows a [CircularProgressIndicator] when [isLoading] is true.
/// Grays out (no tap) when [isLoading] is true OR [onTap] is null.
///
/// ```dart
/// GradientButton(
///   label: 'Create account',
///   onTap: _submit,
///   isLoading: state.isLoading,
/// )
/// ```
class GradientButton extends StatelessWidget {
  const GradientButton({
    super.key,
    required this.label,
    required this.onTap,
    this.isLoading = false,
    this.height = 50,
    this.borderRadius = 8,
    this.gradient = AppColors.gradient,
    this.textStyle,
    this.icon,
  });

  final String label;
  final VoidCallback? onTap;
  final bool isLoading;
  final double height;
  final double borderRadius;
  final LinearGradient gradient;
  final TextStyle? textStyle;
  final Widget? icon;

  bool get _isDisabled => isLoading || onTap == null;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: _isDisabled ? null : onTap,
      child: AnimatedOpacity(
        opacity: _isDisabled ? 0.5 : 1.0,
        duration: const Duration(milliseconds: 200),
        child: Container(
          height: height,
          width: double.infinity,
          decoration: BoxDecoration(
            gradient: _isDisabled
                ? const LinearGradient(
                    colors: [AppColors.surfaceVariant, AppColors.surfaceVariant],
                  )
                : gradient,
            borderRadius: BorderRadius.circular(borderRadius),
          ),
          child: Material(
            color: Colors.transparent,
            child: InkWell(
              onTap: _isDisabled ? null : onTap,
              borderRadius: BorderRadius.circular(borderRadius),
              splashColor: Colors.white.withValues(alpha: 0.1),
              highlightColor: Colors.transparent,
              child: Center(
                child: isLoading
                    ? const SizedBox(
                        width: 22,
                        height: 22,
                        child: CircularProgressIndicator(
                          strokeWidth: 2.5,
                          valueColor:
                              AlwaysStoppedAnimation<Color>(Colors.white),
                        ),
                      )
                    : Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          if (icon != null) ...[
                            icon!,
                            const SizedBox(width: 8),
                          ],
                          Text(
                            label,
                            style: textStyle ??
                                const TextStyle(
                                  color: Colors.white,
                                  fontSize: 16,
                                  fontWeight: FontWeight.w600,
                                  letterSpacing: 0,
                                ),
                          ),
                        ],
                      ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
