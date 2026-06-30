import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:tiktok_clone/features/comments/domain/entities/comment_entity.dart';
import 'package:tiktok_clone/features/comments/presentation/providers/comment_provider.dart';
import 'package:timeago/timeago.dart' as timeago;

class CommentsScreen extends ConsumerStatefulWidget {
  final String videoId;
  final bool isCreator;

  const CommentsScreen({
    super.key,
    required this.videoId,
    this.isCreator = false,
  });

  @override
  ConsumerState<CommentsScreen> createState() => _CommentsScreenState();
}

class _CommentsScreenState extends ConsumerState<CommentsScreen> {
  final _textController = TextEditingController();
  final _scrollController = ScrollController();
  final _focusNode = FocusNode();
  String? _replyToId;
  String? _replyToUsername;

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_onScroll);
  }

  void _onScroll() {
    if (_scrollController.position.pixels >=
        _scrollController.position.maxScrollExtent - 200) {
      ref.read(commentProvider(widget.videoId).notifier).loadMore();
    }
  }

  @override
  void dispose() {
    _textController.dispose();
    _scrollController.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  void _setReply(String commentId, String username) {
    setState(() {
      _replyToId = commentId;
      _replyToUsername = username;
      _textController.text = '@$username ';
    });
    _focusNode.requestFocus();
  }

  void _clearReply() {
    setState(() {
      _replyToId = null;
      _replyToUsername = null;
      _textController.clear();
    });
  }

  Future<void> _sendComment() async {
    final text = _textController.text.trim();
    if (text.isEmpty) return;

    await ref.read(commentProvider(widget.videoId).notifier).createComment(
          content: text,
          parentId: _replyToId,
        );

    _clearReply();
    _focusNode.unfocus();
  }

  void _showContextMenu(BuildContext context, CommentEntity comment) {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (_) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const SizedBox(height: 8),
            Container(
              width: 40,
              height: 4,
              decoration: BoxDecoration(
                color: Colors.white24,
                borderRadius: BorderRadius.circular(2),
              ),
            ),
            const SizedBox(height: 16),
            if (widget.isCreator) ...[
              _MenuItem(
                icon: Icons.push_pin,
                label: comment.isPinned ? 'Unpin comment' : 'Pin comment',
                onTap: () {
                  Navigator.pop(context);
                  ref
                      .read(commentProvider(widget.videoId).notifier)
                      .pinComment(comment.id);
                },
              ),
            ],
            _MenuItem(
              icon: Icons.delete_outline,
              label: 'Delete comment',
              color: Colors.redAccent,
              onTap: () {
                Navigator.pop(context);
                ref
                    .read(commentProvider(widget.videoId).notifier)
                    .deleteComment(comment.id);
              },
            ),
            _MenuItem(
              icon: Icons.flag_outlined,
              label: 'Report comment',
              onTap: () {
                Navigator.pop(context);
                _showReportDialog(comment.id);
              },
            ),
            const SizedBox(height: 8),
          ],
        ),
      ),
    );
  }

  void _showReportDialog(String commentId) {
    final reasons = [
      'Spam',
      'Harassment',
      'Hate speech',
      'Misinformation',
      'Other',
    ];
    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: const Color(0xFF1A1A1A),
        title: const Text('Report Comment',
            style: TextStyle(color: Colors.white)),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: reasons
              .map(
                (r) => ListTile(
                  title:
                      Text(r, style: const TextStyle(color: Colors.white70)),
                  onTap: () {
                    Navigator.pop(ctx);
                    ref
                        .read(commentRepositoryProvider)
                        .reportComment(id: commentId, reason: r);
                  },
                ),
              )
              .toList(),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final asyncState = ref.watch(commentProvider(widget.videoId));

    return Scaffold(
      backgroundColor: const Color(0xFF1A1A1A),
      body: SafeArea(
        child: Column(
          children: [
            // ── Handle bar ──────────────────────────────────────────────────
            const SizedBox(height: 8),
            Container(
              width: 40,
              height: 4,
              decoration: BoxDecoration(
                color: Colors.white24,
                borderRadius: BorderRadius.circular(2),
              ),
            ),

            // ── Header ──────────────────────────────────────────────────────
            asyncState.when(
              loading: () => const _Header(count: 0),
              error: (_, __) => const _Header(count: 0),
              data: (state) => _Header(count: state.comments.length),
            ),
            const Divider(color: Colors.white12, height: 1),

            // ── Comment list ────────────────────────────────────────────────
            Expanded(
              child: asyncState.when(
                loading: () => const Center(
                  child:
                      CircularProgressIndicator(color: Color(0xFFFF0050)),
                ),
                error: (e, _) => Center(
                  child: Text(
                    e.toString(),
                    style: const TextStyle(color: Colors.white54),
                  ),
                ),
                data: (state) {
                  if (state.error != null && state.comments.isEmpty) {
                    return Center(
                      child: Text(
                        state.error!,
                        style: const TextStyle(color: Colors.white54),
                      ),
                    );
                  }
                  if (state.comments.isEmpty) {
                    return const Center(
                      child: Text(
                        'No comments yet. Be the first!',
                        style: TextStyle(color: Colors.white54),
                      ),
                    );
                  }
                  return ListView.builder(
                    controller: _scrollController,
                    padding: const EdgeInsets.symmetric(vertical: 8),
                    itemCount: state.comments.length +
                        (state.isLoadingMore ? 1 : 0),
                    itemBuilder: (context, index) {
                      if (index == state.comments.length) {
                        return const Padding(
                          padding: EdgeInsets.all(16),
                          child: Center(
                            child: CircularProgressIndicator(
                              color: Color(0xFFFF0050),
                              strokeWidth: 2,
                            ),
                          ),
                        );
                      }
                      final comment = state.comments[index];
                      return _CommentTile(
                        comment: comment,
                        isCreator: widget.isCreator,
                        onReply: () =>
                            _setReply(comment.id, comment.username),
                        onLike: () => ref
                            .read(commentProvider(widget.videoId).notifier)
                            .toggleLike(comment.id),
                        onLongPress: () =>
                            _showContextMenu(context, comment),
                      );
                    },
                  );
                },
              ),
            ),

            // ── Reply indicator ─────────────────────────────────────────────
            if (_replyToUsername != null)
              Container(
                color: Colors.white10,
                padding:
                    const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
                child: Row(
                  children: [
                    Text(
                      'Replying to @$_replyToUsername',
                      style: const TextStyle(
                        color: Colors.white54,
                        fontSize: 12,
                      ),
                    ),
                    const Spacer(),
                    GestureDetector(
                      onTap: _clearReply,
                      child: const Icon(Icons.close,
                          color: Colors.white54, size: 16),
                    ),
                  ],
                ),
              ),

            // ── Input bar ───────────────────────────────────────────────────
            _CommentInputBar(
              controller: _textController,
              focusNode: _focusNode,
              onSend: _sendComment,
            ),
          ],
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Sub-widgets
// ---------------------------------------------------------------------------

class _Header extends StatelessWidget {
  final int count;
  const _Header({required this.count});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      child: Row(
        children: [
          Text(
            '$count Comments',
            style: const TextStyle(
              color: Colors.white,
              fontSize: 16,
              fontWeight: FontWeight.w600,
            ),
          ),
          const Spacer(),
          GestureDetector(
            onTap: () => Navigator.of(context).pop(),
            child: const Icon(Icons.close, color: Colors.white70),
          ),
        ],
      ),
    );
  }
}

class _MenuItem extends StatelessWidget {
  final IconData icon;
  final String label;
  final VoidCallback onTap;
  final Color? color;

  const _MenuItem({
    required this.icon,
    required this.label,
    required this.onTap,
    this.color,
  });

  @override
  Widget build(BuildContext context) {
    return ListTile(
      leading: Icon(icon, color: color ?? Colors.white70),
      title: Text(label, style: TextStyle(color: color ?? Colors.white70)),
      onTap: onTap,
    );
  }
}

class _CommentTile extends StatelessWidget {
  final CommentEntity comment;
  final bool isCreator;
  final VoidCallback onReply;
  final VoidCallback onLike;
  final VoidCallback onLongPress;

  const _CommentTile({
    required this.comment,
    required this.isCreator,
    required this.onReply,
    required this.onLike,
    required this.onLongPress,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onLongPress: onLongPress,
      child: Padding(
        padding: EdgeInsets.only(
          left: comment.parentId != null ? 52 : 12,
          right: 12,
          top: 8,
          bottom: 8,
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Avatar
            CircleAvatar(
              radius: comment.parentId != null ? 14 : 18,
              backgroundImage: comment.avatarUrl.isNotEmpty
                  ? NetworkImage(comment.avatarUrl)
                  : null,
              backgroundColor: const Color(0xFF333333),
              child: comment.avatarUrl.isEmpty
                  ? Text(
                      comment.username.isNotEmpty
                          ? comment.username[0].toUpperCase()
                          : '?',
                      style: const TextStyle(
                          color: Colors.white, fontSize: 12),
                    )
                  : null,
            ),
            const SizedBox(width: 10),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Username + pin badge
                  Row(
                    children: [
                      Text(
                        comment.username,
                        style: const TextStyle(
                          color: Colors.white,
                          fontWeight: FontWeight.w600,
                          fontSize: 13,
                        ),
                      ),
                      if (comment.isPinned) ...[
                        const SizedBox(width: 6),
                        const Text(
                          '\u{1F4CC}',
                          style: TextStyle(fontSize: 12),
                        ),
                        const SizedBox(width: 4),
                        const Text(
                          'Pinned',
                          style: TextStyle(
                            color: Color(0xFFFF0050),
                            fontSize: 11,
                            fontWeight: FontWeight.w500,
                          ),
                        ),
                      ],
                    ],
                  ),
                  const SizedBox(height: 2),
                  // Content
                  Text(
                    comment.content,
                    style: const TextStyle(
                      color: Colors.white70,
                      fontSize: 14,
                    ),
                  ),
                  const SizedBox(height: 6),
                  // Meta row
                  Row(
                    children: [
                      Text(
                        timeago.format(comment.createdAt),
                        style: const TextStyle(
                          color: Colors.white38,
                          fontSize: 12,
                        ),
                      ),
                      const SizedBox(width: 16),
                      GestureDetector(
                        onTap: onReply,
                        child: const Text(
                          'Reply',
                          style: TextStyle(
                            color: Colors.white54,
                            fontSize: 12,
                            fontWeight: FontWeight.w500,
                          ),
                        ),
                      ),
                      if (comment.replyCount > 0) ...[
                        const SizedBox(width: 12),
                        Text(
                          '${comment.replyCount} ${comment.replyCount == 1 ? 'reply' : 'replies'}',
                          style: const TextStyle(
                            color: Color(0xFFFF0050),
                            fontSize: 12,
                          ),
                        ),
                      ],
                    ],
                  ),
                ],
              ),
            ),
            // Like button
            Column(
              children: [
                GestureDetector(
                  onTap: onLike,
                  child: Icon(
                    comment.isLiked
                        ? Icons.favorite
                        : Icons.favorite_border,
                    color: comment.isLiked
                        ? const Color(0xFFFF0050)
                        : Colors.white54,
                    size: 18,
                  ),
                ),
                if (comment.likeCount > 0)
                  Text(
                    '${comment.likeCount}',
                    style: const TextStyle(
                      color: Colors.white54,
                      fontSize: 11,
                    ),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _CommentInputBar extends StatelessWidget {
  final TextEditingController controller;
  final FocusNode focusNode;
  final VoidCallback onSend;

  const _CommentInputBar({
    required this.controller,
    required this.focusNode,
    required this.onSend,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      color: const Color(0xFF1A1A1A),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      child: Row(
        children: [
          // Emoji button
          GestureDetector(
            onTap: () {
              // Emoji picker integration point
            },
            child: const Padding(
              padding: EdgeInsets.symmetric(horizontal: 6),
              child: Text('\u{1F60A}', style: TextStyle(fontSize: 22)),
            ),
          ),
          // Text field
          Expanded(
            child: Container(
              constraints: const BoxConstraints(maxHeight: 120),
              decoration: BoxDecoration(
                color: Colors.white10,
                borderRadius: BorderRadius.circular(24),
              ),
              child: TextField(
                controller: controller,
                focusNode: focusNode,
                style:
                    const TextStyle(color: Colors.white, fontSize: 14),
                maxLines: null,
                textInputAction: TextInputAction.newline,
                decoration: const InputDecoration(
                  hintText: 'Add a comment...',
                  hintStyle: TextStyle(color: Colors.white38),
                  contentPadding: EdgeInsets.symmetric(
                      horizontal: 16, vertical: 10),
                  border: InputBorder.none,
                ),
              ),
            ),
          ),
          const SizedBox(width: 8),
          // Send button
          GestureDetector(
            onTap: onSend,
            child: Container(
              width: 36,
              height: 36,
              decoration: const BoxDecoration(
                color: Color(0xFFFF0050),
                shape: BoxShape.circle,
              ),
              child:
                  const Icon(Icons.send, color: Colors.white, size: 18),
            ),
          ),
        ],
      ),
    );
  }
}