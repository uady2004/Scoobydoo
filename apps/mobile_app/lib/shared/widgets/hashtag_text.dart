import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/app_text_styles.dart';

/// Renders a string containing hashtags and @-mentions as a tappable
/// [RichText] widget.
///
/// - `#word`    → teal, taps navigate to `/hashtag/<tag>`
/// - `@word`    → white bold, taps navigate to `/profile/<username>`
/// - plain text → [AppColors.textSecondary] at the default body size
///
/// Navigation is performed via [Navigator] named routes. If your project uses
/// go_router, pass an [onHashtagTap] / [onMentionTap] callback instead of
/// relying on the default pushNamed behaviour.
///
/// ```dart
/// HashtagText(
///   text: '#flutter is amazing, check out @johndev',
///   onHashtagTap: (tag) => context.push('/hashtag/$tag'),
///   onMentionTap: (username) => context.push('/profile/$username'),
/// )
/// ```
class HashtagText extends StatefulWidget {
  const HashtagText({
    super.key,
    required this.text,
    this.style,
    this.maxLines,
    this.overflow = TextOverflow.visible,
    this.onHashtagTap,
    this.onMentionTap,
  });

  final String text;

  /// Base style applied to plain text. Defaults to [AppTextStyles.bodySecondary].
  final TextStyle? style;
  final int? maxLines;
  final TextOverflow overflow;

  /// Called with the tag word (without `#`) when a hashtag is tapped.
  final void Function(String tag)? onHashtagTap;

  /// Called with the username (without `@`) when a mention is tapped.
  final void Function(String username)? onMentionTap;

  @override
  State<HashtagText> createState() => _HashtagTextState();
}

class _HashtagTextState extends State<HashtagText> {
  // Holds all gesture recognisers so we can dispose them on widget teardown.
  final List<TapGestureRecognizer> _recognizers = [];

  @override
  void dispose() {
    for (final r in _recognizers) {
      r.dispose();
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    // Dispose previous recognisers before rebuilding.
    for (final r in _recognizers) {
      r.dispose();
    }
    _recognizers.clear();

    final spans = _buildSpans(widget.text);

    return RichText(
      text: TextSpan(children: spans),
      maxLines: widget.maxLines,
      overflow: widget.overflow,
    );
  }

  List<InlineSpan> _buildSpans(String text) {
    final baseStyle = widget.style ?? AppTextStyles.bodySecondary;
    final spans = <InlineSpan>[];

    // Split on whitespace while preserving the delimiter tokens.
    final tokens = text.split(RegExp(r'(?<=\s)|(?=\s)'));

    for (final token in tokens) {
      if (token.startsWith('#') && token.length > 1) {
        final tag = _stripPunctuation(token.substring(1));
        if (tag.isNotEmpty) {
          final recognizer = TapGestureRecognizer()
            ..onTap = () {
              if (widget.onHashtagTap != null) {
                widget.onHashtagTap!(tag);
              } else {
                Navigator.of(context).pushNamed('/hashtag/$tag');
              }
            };
          _recognizers.add(recognizer);
          spans.add(TextSpan(
            text: '#$tag',
            style: baseStyle.copyWith(color: AppColors.secondary),
            recognizer: recognizer,
          ));
          // Append any trailing punctuation as plain text.
          final trailing = token.substring(tag.length + 1);
          if (trailing.isNotEmpty) {
            spans.add(TextSpan(text: trailing, style: baseStyle));
          }
          continue;
        }
      }

      if (token.startsWith('@') && token.length > 1) {
        final username = _stripPunctuation(token.substring(1));
        if (username.isNotEmpty) {
          final recognizer = TapGestureRecognizer()
            ..onTap = () {
              if (widget.onMentionTap != null) {
                widget.onMentionTap!(username);
              } else {
                Navigator.of(context).pushNamed('/profile/$username');
              }
            };
          _recognizers.add(recognizer);
          spans.add(TextSpan(
            text: '@$username',
            style: baseStyle.copyWith(
              color: Colors.white,
              fontWeight: FontWeight.w600,
            ),
            recognizer: recognizer,
          ));
          final trailing = token.substring(username.length + 1);
          if (trailing.isNotEmpty) {
            spans.add(TextSpan(text: trailing, style: baseStyle));
          }
          continue;
        }
      }

      spans.add(TextSpan(text: token, style: baseStyle));
    }

    return spans;
  }

  /// Strips trailing punctuation from a token so `#flutter!` → `flutter`.
  String _stripPunctuation(String word) {
    return word.replaceAll(RegExp(r'[^\w]$'), '');
  }
}
