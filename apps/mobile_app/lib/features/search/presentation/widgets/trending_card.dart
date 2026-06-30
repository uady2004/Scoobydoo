import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

class TrendingCard extends StatelessWidget {
  final Map<String, dynamic> item;

  const TrendingCard({super.key, required this.item});

  @override
  Widget build(BuildContext context) {
    final tag = item['tag'] as String? ?? '';
    final videoCount = (item['video_count'] as num?)?.toInt() ?? 0;
    final thumbnailUrl = item['thumbnail_url'] as String? ?? '';

    return GestureDetector(
      onTap: () => context.push('/hashtag/${tag.replaceAll('#', '')}'),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(8),
        child: Stack(
          fit: StackFit.expand,
          children: [
            // Thumbnail background
            thumbnailUrl.isNotEmpty
                ? Image.network(
                    thumbnailUrl,
                    fit: BoxFit.cover,
                    errorBuilder: (_, __, ___) => Container(
                      color: const Color(0xFF2A2A2A),
                    ),
                  )
                : Container(
                    decoration: BoxDecoration(
                      gradient: LinearGradient(
                        begin: Alignment.topLeft,
                        end: Alignment.bottomRight,
                        colors: [
                          const Color(0xFFFF0050).withValues(alpha: 0.8),
                          const Color(0xFF00F2EA).withValues(alpha: 0.8),
                        ],
                      ),
                    ),
                  ),

            // Gradient overlay — bottom half
            Positioned.fill(
              child: DecoratedBox(
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    begin: Alignment.topCenter,
                    end: Alignment.bottomCenter,
                    stops: const [0.35, 1.0],
                    colors: [
                      Colors.transparent,
                      Colors.black.withValues(alpha: 0.75),
                    ],
                  ),
                ),
              ),
            ),

            // Text overlay
            Positioned(
              bottom: 8,
              left: 8,
              right: 8,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    tag.startsWith('#') ? tag : '#$tag',
                    style: const TextStyle(
                      color: Colors.white,
                      fontWeight: FontWeight.w700,
                      fontSize: 13,
                      shadows: [
                        Shadow(blurRadius: 4, color: Colors.black54),
                      ],
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 2),
                  Text(
                    '${_formatCount(videoCount)} videos',
                    style: const TextStyle(
                      color: Colors.white70,
                      fontSize: 11,
                      shadows: [
                        Shadow(blurRadius: 4, color: Colors.black54),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  String _formatCount(int count) {
    if (count >= 1000000000) {
      return '${(count / 1000000000).toStringAsFixed(1)}B';
    }
    if (count >= 1000000) return '${(count / 1000000).toStringAsFixed(1)}M';
    if (count >= 1000) return '${(count / 1000).toStringAsFixed(1)}K';
    return '$count';
  }
}
