import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';

/// A centred empty-state layout: icon, title, subtitle, and an optional
/// retry action.
///
/// ```dart
/// EmptyStateWidget(
///   icon: Icons.videocam_off_outlined,
///   title: 'No videos yet',
///   subtitle: 'Videos you post will appear here.',
///   retryLabel: 'Upload your first video',
///   onRetry: () => context.push('/upload'),
/// )
/// ```
class EmptyStateWidget extends StatelessWidget {
  const EmptyStateWidget({
    super.key,
    required this.icon,
    required this.title,
    required this.subtitle,
    this.retryLabel,
    this.onRetry,
    this.iconColor,
  });

  final IconData icon;
  final String title;
  final String subtitle;

  /// Label for the optional action button. Requires [onRetry] to be set.
  final String? retryLabel;
  final VoidCallback? onRetry;

  /// Defaults to [Colors.grey.shade700] when null.
  final Color? iconColor;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              icon,
              size: 72,
              color: iconColor ?? Colors.grey[700],
            ),
            const SizedBox(height: 16),
            Text(
              title,
              textAlign: TextAlign.center,
              style: TextStyle(
                color: Colors.grey[400],
                fontSize: 16,
                fontWeight: FontWeight.w500,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              subtitle,
              textAlign: TextAlign.center,
              style: TextStyle(
                color: Colors.grey[700],
                fontSize: 13,
                fontWeight: FontWeight.w400,
                height: 1.5,
              ),
            ),
            if (retryLabel != null && onRetry != null) ...[
              const SizedBox(height: 20),
              TextButton(
                onPressed: onRetry,
                style: TextButton.styleFrom(
                  foregroundColor: AppColors.primary,
                  textStyle: const TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                child: Text(retryLabel!),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
