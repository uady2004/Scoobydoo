import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:tiktok_clone/features/followers/presentation/providers/followers_provider.dart';

class UserListTile extends ConsumerWidget {
  final Map<String, dynamic> user;

  const UserListTile({super.key, required this.user});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final userId = user['id'] as String? ?? '';
    final username = user['username'] as String? ?? '';
    final displayName = user['display_name'] as String? ?? username;
    final avatarUrl = user['avatar_url'] as String? ?? '';
    final followerCount = (user['follower_count'] as num?)?.toInt() ?? 0;
    final followState = ref.watch(followProvider(userId));

    return ListTile(
      contentPadding:
          const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
      onTap: () => context.push('/profile/$userId'),
      leading: GestureDetector(
        onTap: () => context.push('/profile/$userId'),
        child: CircleAvatar(
          radius: 24,
          backgroundImage:
              avatarUrl.isNotEmpty ? NetworkImage(avatarUrl) : null,
          backgroundColor: const Color(0xFF2A2A2A),
          child: avatarUrl.isEmpty
              ? Text(
                  username.isNotEmpty ? username[0].toUpperCase() : '?',
                  style: const TextStyle(
                      color: Colors.white,
                      fontSize: 16,
                      fontWeight: FontWeight.w600),
                )
              : null,
        ),
      ),
      title: Text(
        displayName,
        style: const TextStyle(
          color: Colors.white,
          fontWeight: FontWeight.w600,
          fontSize: 14,
        ),
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
      ),
      subtitle: Text(
        '@$username  •  ${_formatCount(followerCount)} followers',
        style: const TextStyle(color: Colors.white54, fontSize: 12),
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
      ),
      trailing: _FollowButton(
        userId: userId,
        isFollowing: followState.isFollowing,
        isLoading: followState.isLoading,
        onTap: () =>
            ref.read(followProvider(userId).notifier).toggle(),
      ),
    );
  }

  String _formatCount(int count) {
    if (count >= 1000000) return '${(count / 1000000).toStringAsFixed(1)}M';
    if (count >= 1000) return '${(count / 1000).toStringAsFixed(1)}K';
    return '$count';
  }
}

class _FollowButton extends StatelessWidget {
  final String userId;
  final bool isFollowing;
  final bool isLoading;
  final VoidCallback onTap;

  const _FollowButton({
    required this.userId,
    required this.isFollowing,
    required this.isLoading,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    if (isLoading) {
      return const SizedBox(
        width: 80,
        height: 34,
        child: Center(
          child: SizedBox(
            width: 18,
            height: 18,
            child: CircularProgressIndicator(
              strokeWidth: 2,
              color: Color(0xFFFF0050),
            ),
          ),
        ),
      );
    }

    if (isFollowing) {
      return OutlinedButton(
        onPressed: onTap,
        style: OutlinedButton.styleFrom(
          foregroundColor: Colors.white70,
          side: const BorderSide(color: Colors.white24),
          padding: const EdgeInsets.symmetric(horizontal: 16),
          minimumSize: const Size(80, 34),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(6),
          ),
        ),
        child: const Text('Following', style: TextStyle(fontSize: 13)),
      );
    }

    return ElevatedButton(
      onPressed: onTap,
      style: ElevatedButton.styleFrom(
        backgroundColor: const Color(0xFFFF0050),
        foregroundColor: Colors.white,
        padding: const EdgeInsets.symmetric(horizontal: 16),
        minimumSize: const Size(80, 34),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(6),
        ),
        elevation: 0,
      ),
      child: const Text('Follow', style: TextStyle(fontSize: 13)),
    );
  }
}
