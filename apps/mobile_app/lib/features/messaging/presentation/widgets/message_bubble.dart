import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:tiktok_clone/features/messaging/domain/entities/message_entity.dart';

class MessageBubble extends StatelessWidget {
  final MessageEntity message;
  final bool isOwn;
  final bool showAvatar;
  final String currentUserId;
  final VoidCallback? onReply;
  final VoidCallback? onDelete;
  final VoidCallback? onForward;

  const MessageBubble({
    super.key,
    required this.message,
    required this.isOwn,
    required this.currentUserId,
    this.showAvatar = false,
    this.onReply,
    this.onDelete,
    this.onForward,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onLongPress: () => _showOptions(context),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 2),
        child: Row(
          mainAxisAlignment:
              isOwn ? MainAxisAlignment.end : MainAxisAlignment.start,
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            if (!isOwn) _buildOtherAvatar(),
            const SizedBox(width: 6),
            Flexible(child: _buildBubble(context)),
            if (isOwn) const SizedBox(width: 4),
          ],
        ),
      ),
    );
  }

  Widget _buildOtherAvatar() {
    if (!showAvatar) return const SizedBox(width: 32);
    return CircleAvatar(
      radius: 16,
      backgroundColor: const Color(0xFF2A2A2A),
      backgroundImage: message.senderAvatarUrl != null
          ? CachedNetworkImageProvider(message.senderAvatarUrl!)
          : null,
      child: message.senderAvatarUrl == null
          ? Text(
              message.senderUsername.isNotEmpty
                  ? message.senderUsername[0].toUpperCase()
                  : '?',
              style: const TextStyle(
                color: Colors.white,
                fontSize: 12,
                fontWeight: FontWeight.w600,
              ),
            )
          : null,
    );
  }

  Widget _buildBubble(BuildContext context) {
    return Container(
      constraints: BoxConstraints(
        maxWidth: MediaQuery.of(context).size.width * 0.72,
      ),
      decoration: BoxDecoration(
        gradient: isOwn
            ? const LinearGradient(
                colors: [Color(0xFFFE2C55), Color(0xFFFF6B8A)],
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
              )
            : null,
        color: isOwn ? null : const Color(0xFF1A1A1A),
        borderRadius: BorderRadius.only(
          topLeft: const Radius.circular(18),
          topRight: const Radius.circular(18),
          bottomLeft: Radius.circular(isOwn ? 18 : 4),
          bottomRight: Radius.circular(isOwn ? 4 : 18),
        ),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            if (message.replyToContent != null) _buildReplyPreview(),
            _buildContent(context),
            const SizedBox(height: 4),
            _buildFooter(),
          ],
        ),
      ),
    );
  }

  Widget _buildReplyPreview() {
    return Container(
      margin: const EdgeInsets.only(bottom: 6),
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
      decoration: BoxDecoration(
        color: Colors.black.withValues(alpha: 0.25),
        borderRadius: BorderRadius.circular(8),
        border: const Border(
          left: BorderSide(color: Color(0xFFFE2C55), width: 3),
        ),
      ),
      child: Text(
        message.replyToContent ?? '',
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: const TextStyle(color: Colors.white70, fontSize: 12),
      ),
    );
  }

  Widget _buildContent(BuildContext context) {
    switch (message.type) {
      case MessageType.image:
        return _buildImageContent();
      case MessageType.voice:
        return _buildVoiceContent();
      case MessageType.video:
        return _buildVideoContent();
      case MessageType.gift:
        return _buildGiftContent();
      case MessageType.system:
        return _buildSystemContent();
      case MessageType.text:
        return Text(
          message.content,
          style: const TextStyle(
            color: Colors.white,
            fontSize: 15,
            height: 1.4,
          ),
        );
    }
  }

  Widget _buildImageContent() {
    if (message.mediaUrl == null) {
      return const Text('📷 Photo',
          style: TextStyle(color: Colors.white));
    }
    return ClipRRect(
      borderRadius: BorderRadius.circular(10),
      child: CachedNetworkImage(
        imageUrl: message.mediaUrl!,
        height: 200,
        width: double.infinity,
        fit: BoxFit.cover,
        placeholder: (_, __) => Container(
          height: 200,
          color: const Color(0xFF2A2A2A),
          child: const Center(
            child: CircularProgressIndicator(
              valueColor:
                  AlwaysStoppedAnimation(Color(0xFFFE2C55)),
              strokeWidth: 2,
            ),
          ),
        ),
        errorWidget: (_, __, ___) => Container(
          height: 200,
          color: const Color(0xFF2A2A2A),
          child: const Icon(Icons.broken_image, color: Colors.white38),
        ),
      ),
    );
  }

  Widget _buildVoiceContent() {
    return _VoiceMessageContent(message: message);
  }

  Widget _buildVideoContent() {
    return const Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(Icons.play_circle_fill, color: Colors.white, size: 22),
        SizedBox(width: 6),
        Text('Video',
            style: TextStyle(color: Colors.white, fontSize: 14)),
      ],
    );
  }

  Widget _buildGiftContent() {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        const Text('🎁', style: TextStyle(fontSize: 22)),
        const SizedBox(width: 6),
        Text(
          message.content.isNotEmpty ? message.content : 'Gift',
          style: const TextStyle(color: Colors.white, fontSize: 14),
        ),
      ],
    );
  }

  Widget _buildSystemContent() {
    return Text(
      message.content,
      style: const TextStyle(
        color: Colors.white54,
        fontSize: 12,
        fontStyle: FontStyle.italic,
      ),
      textAlign: TextAlign.center,
    );
  }

  Widget _buildFooter() {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          _formatTime(message.createdAt),
          style: const TextStyle(color: Colors.white54, fontSize: 10),
        ),
        if (isOwn) ...[
          const SizedBox(width: 4),
          _buildReadReceipt(),
        ],
      ],
    );
  }

  Widget _buildReadReceipt() {
    if (message.readAt != null) {
      return const _DoubleTick(color: Color(0xFF4FC3F7));
    }
    if (!message.id.startsWith('temp_')) {
      return const _DoubleTick(color: Colors.white38);
    }
    return const Icon(Icons.check, size: 12, color: Colors.white38);
  }

  String _formatTime(DateTime dt) {
    final h = dt.hour.toString().padLeft(2, '0');
    final m = dt.minute.toString().padLeft(2, '0');
    return '$h:$m';
  }

  void _showOptions(BuildContext context) {
    HapticFeedback.mediumImpact();
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (_) => _MessageOptionsSheet(
        message: message,
        isOwn: isOwn,
        onReply: onReply,
        onDelete: onDelete,
        onForward: onForward,
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Voice message content
// ---------------------------------------------------------------------------

class _VoiceMessageContent extends StatefulWidget {
  final MessageEntity message;
  const _VoiceMessageContent({required this.message});

  @override
  State<_VoiceMessageContent> createState() =>
      _VoiceMessageContentState();
}

class _VoiceMessageContentState extends State<_VoiceMessageContent> {
  bool _playing = false;

  static const _barHeights = [
    8.0, 14.0, 20.0, 12.0, 18.0, 24.0, 10.0, 16.0, 22.0, 8.0,
    14.0, 18.0, 10.0, 20.0, 6.0, 16.0, 12.0, 22.0, 8.0, 14.0,
  ];

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        GestureDetector(
          onTap: () => setState(() => _playing = !_playing),
          child: Container(
            width: 36,
            height: 36,
            decoration: const BoxDecoration(
              color: Colors.white24,
              shape: BoxShape.circle,
            ),
            child: Icon(
              _playing ? Icons.pause : Icons.play_arrow,
              color: Colors.white,
              size: 20,
            ),
          ),
        ),
        const SizedBox(width: 8),
        Row(
          crossAxisAlignment: CrossAxisAlignment.center,
          children: List.generate(_barHeights.length, (i) {
            return Container(
              margin: const EdgeInsets.symmetric(horizontal: 1),
              width: 3,
              height: _barHeights[i],
              decoration: BoxDecoration(
                color: Colors.white
                    .withValues(alpha: _playing ? 1.0 : 0.5),
                borderRadius: BorderRadius.circular(2),
              ),
            );
          }),
        ),
        const SizedBox(width: 8),
        const Text(
          '0:12',
          style: TextStyle(color: Colors.white70, fontSize: 12),
        ),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Double tick
// ---------------------------------------------------------------------------

class _DoubleTick extends StatelessWidget {
  final Color color;
  const _DoubleTick({required this.color});

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(Icons.check, size: 12, color: color),
        Transform.translate(
          offset: const Offset(-5, 0),
          child: Icon(Icons.check, size: 12, color: color),
        ),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Options bottom sheet
// ---------------------------------------------------------------------------

class _MessageOptionsSheet extends StatelessWidget {
  final MessageEntity message;
  final bool isOwn;
  final VoidCallback? onReply;
  final VoidCallback? onDelete;
  final VoidCallback? onForward;

  const _MessageOptionsSheet({
    required this.message,
    required this.isOwn,
    this.onReply,
    this.onDelete,
    this.onForward,
  });

  @override
  Widget build(BuildContext context) {
    const reactions = ['❤️', '😂', '😮', '😢', '😡', '👍', '🔥', '👏'];

    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // ── Handle bar ───────────────────────────────────────────
            Container(
              width: 40,
              height: 4,
              decoration: BoxDecoration(
                color: Colors.white24,
                borderRadius: BorderRadius.circular(2),
              ),
            ),
            const SizedBox(height: 16),

            // ── Message preview ──────────────────────────────────────
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: Colors.white.withValues(alpha: 0.06),
                borderRadius: BorderRadius.circular(12),
              ),
              child: Text(
                message.content.isEmpty ? '📷 Media' : message.content,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                    color: Colors.white70, fontSize: 13),
              ),
            ),
            const SizedBox(height: 16),

            // ── Reaction row ─────────────────────────────────────────
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceEvenly,
              children: reactions.map((emoji) {
                return GestureDetector(
                  onTap: () {
                    Navigator.pop(context);
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(
                        content: Text('Reacted with $emoji'),
                        backgroundColor: const Color(0xFF1A1A1A),
                        behavior: SnackBarBehavior.floating,
                        duration: const Duration(seconds: 1),
                      ),
                    );
                  },
                  child: Container(
                    width: 44,
                    height: 44,
                    decoration: BoxDecoration(
                      color: Colors.white.withValues(alpha: 0.08),
                      shape: BoxShape.circle,
                    ),
                    child: Center(
                      child: Text(emoji,
                          style: const TextStyle(fontSize: 22)),
                    ),
                  ),
                );
              }).toList(),
            ),
            const SizedBox(height: 12),
            const Divider(color: Color(0xFF2A2A2A), height: 1),

            // ── Reply ────────────────────────────────────────────────
            _OptionTile(
              icon: Icons.reply,
              label: 'Reply',
              onTap: () {
                Navigator.pop(context);
                onReply?.call();
              },
            ),

            // ── Copy (text only) ─────────────────────────────────────
            if (message.type == MessageType.text)
              _OptionTile(
                icon: Icons.copy_outlined,
                label: 'Copy',
                onTap: () {
                  Clipboard.setData(
                      ClipboardData(text: message.content));
                  Navigator.pop(context);
                  ScaffoldMessenger.of(context).showSnackBar(
                    const SnackBar(
                      content: Text('Copied to clipboard'),
                      backgroundColor: Color(0xFF1A1A1A),
                      behavior: SnackBarBehavior.floating,
                      duration: Duration(seconds: 1),
                    ),
                  );
                },
              ),

            // ── Forward ──────────────────────────────────────────────
            _OptionTile(
              icon: Icons.forward_outlined,
              label: 'Forward',
              onTap: () {
                Navigator.pop(context);
                onForward?.call();
              },
            ),

            // ── Message info ─────────────────────────────────────────
            _OptionTile(
              icon: Icons.info_outline,
              label: 'Message info',
              onTap: () {
                Navigator.pop(context);
                _showMessageInfo(context);
              },
            ),

            // ── Delete (own messages only) ───────────────────────────
            if (isOwn)
              _OptionTile(
                icon: Icons.delete_outline,
                label: 'Delete',
                color: const Color(0xFFFE2C55),
                onTap: () {
                  Navigator.pop(context); // close bottom sheet first
                  _confirmDelete(context); // then show dialog
                },
              ),

            const SizedBox(height: 8),
          ],
        ),
      ),
    );
  }

  void _confirmDelete(BuildContext context) {
    showDialog<void>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        backgroundColor: const Color(0xFF1A1A1A),
        title: const Text('Delete message',
            style: TextStyle(color: Colors.white)),
        content: const Text(
          'This message will be deleted for everyone.',
          style: TextStyle(color: Colors.white70),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext), // ← closes dialog only
            child: const Text('Cancel',
                style: TextStyle(color: Colors.white54)),
          ),
          TextButton(
            onPressed: () {
              Navigator.pop(dialogContext); // close dialog
              onDelete?.call();             // then delete
            },
            child: const Text('Delete',
                style: TextStyle(color: Color(0xFFEE1D52))),
          ),
        ],
      ),
    );
  }

  void _showMessageInfo(BuildContext context) {
    showDialog<void>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        backgroundColor: const Color(0xFF1A1A1A),
        title: const Text('Message Info',
            style: TextStyle(color: Colors.white)),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _InfoRow(
                label: 'Sent',
                value: _formatDateTime(message.createdAt)),
            if (message.readAt != null)
              _InfoRow(
                  label: 'Read',
                  value: _formatDateTime(message.readAt!)),
            _InfoRow(
              label: 'Status',
              value: message.readAt != null
                  ? '✓✓ Read'
                  : message.id.startsWith('temp_')
                      ? '⏳ Sending...'
                      : '✓✓ Delivered',
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('Close',
                style: TextStyle(color: Color(0xFFFE2C55))),
          ),
        ],
      ),
    );
  }

  String _formatDateTime(DateTime dt) {
    return '${dt.day}/${dt.month}/${dt.year} '
        '${dt.hour.toString().padLeft(2, '0')}:'
        '${dt.minute.toString().padLeft(2, '0')}';
  }
}

// ---------------------------------------------------------------------------
// Info row
// ---------------------------------------------------------------------------

class _InfoRow extends StatelessWidget {
  final String label;
  final String value;
  const _InfoRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        children: [
          Text('$label: ',
              style: const TextStyle(
                  color: Colors.white54, fontSize: 13)),
          Expanded(
            child: Text(value,
                style: const TextStyle(
                    color: Colors.white, fontSize: 13)),
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Option tile
// ---------------------------------------------------------------------------

class _OptionTile extends StatelessWidget {
  final IconData icon;
  final String label;
  final Color? color;
  final VoidCallback onTap;

  const _OptionTile({
    required this.icon,
    required this.label,
    required this.onTap,
    this.color,
  });

  @override
  Widget build(BuildContext context) {
    final c = color ?? Colors.white;
    return ListTile(
      leading: Icon(icon, color: c, size: 22),
      title: Text(label, style: TextStyle(color: c, fontSize: 15)),
      onTap: onTap,
      contentPadding: EdgeInsets.zero,
      dense: true,
    );
  }
}