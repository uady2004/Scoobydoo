// import 'package:cached_network_image/cached_network_image.dart';
// import 'package:flutter/material.dart';
// import 'package:flutter_riverpod/flutter_riverpod.dart';
// import 'package:go_router/go_router.dart';
// import 'package:tiktok_clone/features/messaging/domain/entities/conversation_entity.dart';
// import 'package:tiktok_clone/features/messaging/domain/entities/message_entity.dart';
// import 'package:tiktok_clone/features/messaging/presentation/providers/messaging_provider.dart';

// class InboxScreen extends ConsumerStatefulWidget {
//   const InboxScreen({super.key});

//   @override
//   ConsumerState<InboxScreen> createState() => _InboxScreenState();
// }

// class _InboxScreenState extends ConsumerState<InboxScreen> {
//   bool _searchVisible = false;
//   String _searchQuery = '';
//   final _searchController = TextEditingController();

//   @override
//   void dispose() {
//     _searchController.dispose();
//     super.dispose();
//   }

//   @override
//   Widget build(BuildContext context) {
//     final inboxAsync = ref.watch(inboxProvider);
//     final currentUserId = ref.watch(currentUserIdProvider);

//     return Scaffold(
//       backgroundColor: Colors.black,
//       appBar: AppBar(
//         backgroundColor: Colors.black,
//         elevation: 0,
//         centerTitle: false,
//         title: _searchVisible
//             ? _SearchBar(
//                 controller: _searchController,
//                 onChanged: (v) => setState(() => _searchQuery = v),
//                 onClose: () => setState(() {
//                   _searchVisible = false;
//                   _searchQuery = '';
//                   _searchController.clear();
//                 }),
//               )
//             : const Text(
//                 'Messages',
//                 style: TextStyle(
//                   color: Colors.white,
//                   fontSize: 20,
//                   fontWeight: FontWeight.w700,
//                 ),
//               ),
//         actions: [
//           if (!_searchVisible)
//             IconButton(
//               icon: const Icon(Icons.search, color: Colors.white),
//               onPressed: () => setState(() => _searchVisible = true),
//             ),
//         ],
//       ),
//       body: inboxAsync.when(
//         loading: () => const Center(
//           child: CircularProgressIndicator(
//             valueColor: AlwaysStoppedAnimation(Color(0xFFFE2C55)),
//             strokeWidth: 2,
//           ),
//         ),
//         error: (err, _) => Center(
//           child: Column(
//             mainAxisSize: MainAxisSize.min,
//             children: [
//               const Icon(Icons.error_outline, color: Colors.white38, size: 48),
//               const SizedBox(height: 12),
//               Text(
//                 err.toString(),
//                 style: const TextStyle(color: Colors.white38, fontSize: 14),
//                 textAlign: TextAlign.center,
//               ),
//               const SizedBox(height: 16),
//               TextButton(
//                 onPressed: () => ref.invalidate(inboxProvider),
//                 child: const Text(
//                   'Try again',
//                   style: TextStyle(color: Color(0xFFFE2C55)),
//                 ),
//               ),
//             ],
//           ),
//         ),
//         data: (conversations) {
//           final filtered = _searchQuery.isEmpty
//               ? conversations
//               : conversations.where((c) {
//                   final name = c.displayName(currentUserId).toLowerCase();
//                   return name.contains(_searchQuery.toLowerCase());
//                 }).toList();

//           if (filtered.isEmpty) {
//             return _EmptyState(hasSearch: _searchQuery.isNotEmpty);
//           }

//           return RefreshIndicator(
//             color: const Color(0xFFFE2C55),
//             backgroundColor: const Color(0xFF1A1A1A),
//             onRefresh: () => ref.read(inboxProvider.notifier).refresh(),
//             child: ListView.builder(
//               itemCount: filtered.length,
//               itemBuilder: (context, index) {
//                 final conv = filtered[index];
//                 return _ConversationTile(
//                   conversation: conv,
//                   currentUserId: currentUserId,
//                   onTap: () => context.push(
//                     '/inbox/chat/${conv.id}',
//                     extra: {
//                       'displayName': conv.displayName(currentUserId),
//                       'avatarUrl': conv.avatarUrl(currentUserId),
//                       'isOnline': conv.isOnline(currentUserId),
//                     },
//                   ),
//                 );
//               },
//             ),
//           );
//         },
//       ),
//     );
//   }
// }

// // ---------------------------------------------------------------------------
// // Search bar
// // ---------------------------------------------------------------------------

// class _SearchBar extends StatelessWidget {
//   final TextEditingController controller;
//   final ValueChanged<String> onChanged;
//   final VoidCallback onClose;

//   const _SearchBar({
//     required this.controller,
//     required this.onChanged,
//     required this.onClose,
//   });

//   @override
//   Widget build(BuildContext context) {
//     return Row(
//       children: [
//         Expanded(
//           child: TextField(
//             controller: controller,
//             autofocus: true,
//             onChanged: onChanged,
//             style: const TextStyle(color: Colors.white, fontSize: 16),
//             decoration: const InputDecoration(
//               hintText: 'Search messages...',
//               hintStyle: TextStyle(color: Colors.white38),
//               border: InputBorder.none,
//               prefixIcon: Icon(Icons.search, color: Colors.white38, size: 20),
//             ),
//           ),
//         ),
//         TextButton(
//           onPressed: onClose,
//           child: const Text(
//             'Cancel',
//             style: TextStyle(color: Color(0xFFFE2C55), fontSize: 14),
//           ),
//         ),
//       ],
//     );
//   }
// }

// // ---------------------------------------------------------------------------
// // Conversation tile
// // ---------------------------------------------------------------------------

// class _ConversationTile extends StatelessWidget {
//   final ConversationEntity conversation;
//   final String currentUserId;
//   final VoidCallback onTap;

//   const _ConversationTile({
//     required this.conversation,
//     required this.currentUserId,
//     required this.onTap,
//   });

//   @override
//   Widget build(BuildContext context) {
//     final name = conversation.displayName(currentUserId);
//     final avatarUrl = conversation.avatarUrl(currentUserId);
//     final isOnline = conversation.isOnline(currentUserId);
//     final hasUnread = conversation.unreadCount > 0;

//     return InkWell(
//       onTap: onTap,
//       splashColor: Colors.white10,
//       highlightColor: Colors.white10,
//       child: Padding(
//         padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
//         child: Row(
//           children: [
//             // Avatar with online dot
//             Stack(
//               children: [
//                 CircleAvatar(
//                   radius: 30,
//                   backgroundColor: const Color(0xFF2A2A2A),
//                   backgroundImage: avatarUrl != null
//                       ? CachedNetworkImageProvider(avatarUrl)
//                       : null,
//                   child: avatarUrl == null
//                       ? Text(
//                           name.isNotEmpty ? name[0].toUpperCase() : '?',
//                           style: const TextStyle(
//                             color: Colors.white,
//                             fontSize: 22,
//                             fontWeight: FontWeight.w600,
//                           ),
//                         )
//                       : null,
//                 ),
//                 if (isOnline)
//                   Positioned(
//                     right: 2,
//                     bottom: 2,
//                     child: Container(
//                       width: 12,
//                       height: 12,
//                       decoration: BoxDecoration(
//                         color: const Color(0xFF00D68F),
//                         shape: BoxShape.circle,
//                         border: Border.all(color: Colors.black, width: 2),
//                       ),
//                     ),
//                   ),
//               ],
//             ),
//             const SizedBox(width: 12),
//             // Content
//             Expanded(
//               child: Column(
//                 crossAxisAlignment: CrossAxisAlignment.start,
//                 children: [
//                   Text(
//                     name,
//                     style: TextStyle(
//                       color: Colors.white,
//                       fontSize: 15,
//                       fontWeight: hasUnread ? FontWeight.w700 : FontWeight.w400,
//                     ),
//                   ),
//                   const SizedBox(height: 3),
//                   Text(
//                     _lastMessagePreview(),
//                     maxLines: 1,
//                     overflow: TextOverflow.ellipsis,
//                     style: TextStyle(
//                       color: hasUnread ? Colors.white70 : Colors.white38,
//                       fontSize: 13,
//                       fontWeight: hasUnread ? FontWeight.w500 : FontWeight.w400,
//                     ),
//                   ),
//                 ],
//               ),
//             ),
//             const SizedBox(width: 8),
//             // Trailing: time + badge
//             Column(
//               crossAxisAlignment: CrossAxisAlignment.end,
//               children: [
//                 Text(
//                   _timeAgo(conversation.lastMessageAt),
//                   style: TextStyle(
//                     color: hasUnread ? const Color(0xFFFE2C55) : Colors.white38,
//                     fontSize: 12,
//                   ),
//                 ),
//                 const SizedBox(height: 4),
//                 if (hasUnread)
//                   Container(
//                     constraints: const BoxConstraints(minWidth: 20),
//                     height: 20,
//                     padding: const EdgeInsets.symmetric(horizontal: 6),
//                     decoration: const BoxDecoration(
//                       color: Color(0xFFFE2C55),
//                       borderRadius: BorderRadius.all(Radius.circular(10)),
//                     ),
//                     child: Center(
//                       child: Text(
//                         conversation.unreadCount > 99
//                             ? '99+'
//                             : '${conversation.unreadCount}',
//                         style: const TextStyle(
//                           color: Colors.white,
//                           fontSize: 11,
//                           fontWeight: FontWeight.w700,
//                         ),
//                       ),
//                     ),
//                   )
//                 else
//                   const SizedBox(height: 20),
//               ],
//             ),
//           ],
//         ),
//       ),
//     );
//   }

//   String _lastMessagePreview() {
//     final msg = conversation.lastMessage;
//     if (msg == null) return 'No messages yet';
//     switch (msg.type) {
//       case MessageType.image:
//         return '📷 Photo';
//       case MessageType.video:
//         return '🎥 Video';
//       case MessageType.voice:
//         return '🎤 Voice';
//       case MessageType.gift:
//         return '🎁 Gift';
//       case MessageType.system:
//         return msg.content;
//       case MessageType.text:
//         final c = msg.content;
//         return c.length > 40 ? '${c.substring(0, 40)}…' : c;
//     }
//   }

//   String _timeAgo(DateTime? dt) {
//     if (dt == null) return '';
//     final diff = DateTime.now().difference(dt);
//     if (diff.inMinutes < 1) return 'now';
//     if (diff.inHours < 1) return '${diff.inMinutes}m';
//     if (diff.inDays < 1) return '${diff.inHours}h';
//     if (diff.inDays < 7) return '${diff.inDays}d';
//     return '${dt.day}/${dt.month}';
//   }
// }

// // ---------------------------------------------------------------------------
// // Empty state
// // ---------------------------------------------------------------------------

// class _EmptyState extends StatelessWidget {
//   final bool hasSearch;
//   const _EmptyState({required this.hasSearch});

//   @override
//   Widget build(BuildContext context) {
//     return Center(
//       child: Padding(
//         padding: const EdgeInsets.symmetric(horizontal: 40),
//         child: Column(
//           mainAxisSize: MainAxisSize.min,
//           children: [
//             const Text('💬', style: TextStyle(fontSize: 56)),
//             const SizedBox(height: 16),
//             Text(
//               hasSearch
//                   ? 'No conversations match your search.'
//                   : 'No messages yet.\nFollow creators and start chatting!',
//               style: const TextStyle(
//                 color: Colors.white54,
//                 fontSize: 15,
//                 height: 1.5,
//               ),
//               textAlign: TextAlign.center,
//             ),
//           ],
//         ),
//       ),
//     );
//   }
// }


import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:tiktok_clone/features/messaging/domain/entities/conversation_entity.dart';
import 'package:tiktok_clone/features/messaging/domain/entities/message_entity.dart';
import 'package:tiktok_clone/features/messaging/presentation/providers/messaging_provider.dart';

class InboxScreen extends ConsumerStatefulWidget {
  const InboxScreen({super.key});

  @override
  ConsumerState<InboxScreen> createState() => _InboxScreenState();
}

class _InboxScreenState extends ConsumerState<InboxScreen> {
  bool _searchVisible = false;
  String _searchQuery = '';
  final _searchController = TextEditingController();

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final inboxAsync = ref.watch(inboxProvider);
    final currentUserId = ref.watch(currentUserIdProvider);

    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        elevation: 0,
        centerTitle: false,
        title: _searchVisible
            ? _SearchBar(
                controller: _searchController,
                onChanged: (v) => setState(() => _searchQuery = v),
                onClose: () => setState(() {
                  _searchVisible = false;
                  _searchQuery = '';
                  _searchController.clear();
                }),
              )
            : const Text(
                'Messages',
                style: TextStyle(
                  color: Colors.white,
                  fontSize: 20,
                  fontWeight: FontWeight.w700,
                ),
              ),
        actions: [
          if (!_searchVisible)
            IconButton(
              icon: const Icon(Icons.search, color: Colors.white),
              onPressed: () => setState(() => _searchVisible = true),
            ),
        ],
      ),
      body: inboxAsync.when(
        loading: () => const Center(
          child: CircularProgressIndicator(
            valueColor: AlwaysStoppedAnimation(Color(0xFFFE2C55)),
            strokeWidth: 2,
          ),
        ),
        error: (err, _) => Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(Icons.error_outline,
                  color: Colors.white38, size: 48),
              const SizedBox(height: 12),
              Text(
                err.toString(),
                style: const TextStyle(
                    color: Colors.white38, fontSize: 14),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 16),
              TextButton(
                onPressed: () => ref.invalidate(inboxProvider),
                child: const Text('Try again',
                    style: TextStyle(color: Color(0xFFFE2C55))),
              ),
            ],
          ),
        ),
        data: (conversations) {
          final filtered = _searchQuery.isEmpty
              ? conversations
              : conversations.where((c) {
                  final name =
                      c.displayName(currentUserId).toLowerCase();
                  return name.contains(_searchQuery.toLowerCase());
                }).toList();

          // Online users for the story-style row
          final onlineConvs = conversations
              .where((c) => c.isOnline(currentUserId))
              .toList();

          return RefreshIndicator(
            color: const Color(0xFFFE2C55),
            backgroundColor: const Color(0xFF1A1A1A),
            onRefresh: () => ref.read(inboxProvider.notifier).refresh(),
            child: CustomScrollView(
              slivers: [
                // ── Online users row ──────────────────────────────────
                if (onlineConvs.isNotEmpty && _searchQuery.isEmpty)
                  SliverToBoxAdapter(
                    child: _OnlineUsersRow(
                      conversations: onlineConvs,
                      currentUserId: currentUserId,
                      onTap: (conv) => context.push(
                        '/inbox/chat/${conv.id}',
                        extra: {
                          'displayName':
                              conv.displayName(currentUserId),
                          'avatarUrl': conv.avatarUrl(currentUserId),
                          'isOnline': conv.isOnline(currentUserId),
                        },
                      ),
                    ),
                  ),

                // ── Divider ───────────────────────────────────────────
                if (onlineConvs.isNotEmpty && _searchQuery.isEmpty)
                  const SliverToBoxAdapter(
                    child: Divider(
                        color: Colors.white12, height: 1),
                  ),

                // ── Conversation list ─────────────────────────────────
                filtered.isEmpty
                    ? SliverFillRemaining(
                        child:
                            _EmptyState(hasSearch: _searchQuery.isNotEmpty),
                      )
                    : SliverList(
                        delegate: SliverChildBuilderDelegate(
                          (context, index) {
                            final conv = filtered[index];
                            return _ConversationTile(
                              conversation: conv,
                              currentUserId: currentUserId,
                              onTap: () => context.push(
                                '/inbox/chat/${conv.id}',
                                extra: {
                                  'displayName':
                                      conv.displayName(currentUserId),
                                  'avatarUrl':
                                      conv.avatarUrl(currentUserId),
                                  'isOnline':
                                      conv.isOnline(currentUserId),
                                },
                              ),
                            );
                          },
                          childCount: filtered.length,
                        ),
                      ),
              ],
            ),
          );
        },
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Online users horizontal row (Instagram-style)
// ---------------------------------------------------------------------------

class _OnlineUsersRow extends StatelessWidget {
  final List<ConversationEntity> conversations;
  final String currentUserId;
  final void Function(ConversationEntity) onTap;

  const _OnlineUsersRow({
    required this.conversations,
    required this.currentUserId,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Padding(
          padding: EdgeInsets.fromLTRB(16, 12, 16, 8),
          child: Text(
            'Active Now',
            style: TextStyle(
              color: Colors.white,
              fontSize: 14,
              fontWeight: FontWeight.w600,
            ),
          ),
        ),
        SizedBox(
          height: 90,
          child: ListView.builder(
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.symmetric(horizontal: 12),
            itemCount: conversations.length,
            itemBuilder: (context, index) {
              final conv = conversations[index];
              final name = conv.displayName(currentUserId);
              final avatarUrl = conv.avatarUrl(currentUserId);

              return GestureDetector(
                onTap: () => onTap(conv),
                child: Container(
                  width: 68,
                  margin: const EdgeInsets.only(right: 8),
                  child: Column(
                    children: [
                      Stack(
                        children: [
                          CircleAvatar(
                            radius: 28,
                            backgroundColor: const Color(0xFF2A2A2A),
                            backgroundImage: avatarUrl != null
                                ? CachedNetworkImageProvider(avatarUrl)
                                : null,
                            child: avatarUrl == null
                                ? Text(
                                    name.isNotEmpty
                                        ? name[0].toUpperCase()
                                        : '?',
                                    style: const TextStyle(
                                      color: Colors.white,
                                      fontSize: 20,
                                      fontWeight: FontWeight.w600,
                                    ),
                                  )
                                : null,
                          ),
                          // Green online dot
                          Positioned(
                            right: 2,
                            bottom: 2,
                            child: Container(
                              width: 14,
                              height: 14,
                              decoration: BoxDecoration(
                                color: const Color(0xFF00D68F),
                                shape: BoxShape.circle,
                                border: Border.all(
                                    color: Colors.black, width: 2.5),
                              ),
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 4),
                      Text(
                        name,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        textAlign: TextAlign.center,
                        style: const TextStyle(
                          color: Colors.white70,
                          fontSize: 11,
                        ),
                      ),
                    ],
                  ),
                ),
              );
            },
          ),
        ),
        const SizedBox(height: 8),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Search bar
// ---------------------------------------------------------------------------

class _SearchBar extends StatelessWidget {
  final TextEditingController controller;
  final ValueChanged<String> onChanged;
  final VoidCallback onClose;

  const _SearchBar({
    required this.controller,
    required this.onChanged,
    required this.onClose,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: TextField(
            controller: controller,
            autofocus: true,
            onChanged: onChanged,
            style: const TextStyle(color: Colors.white, fontSize: 16),
            decoration: const InputDecoration(
              hintText: 'Search messages...',
              hintStyle: TextStyle(color: Colors.white38),
              border: InputBorder.none,
              prefixIcon:
                  Icon(Icons.search, color: Colors.white38, size: 20),
            ),
          ),
        ),
        TextButton(
          onPressed: onClose,
          child: const Text(
            'Cancel',
            style: TextStyle(color: Color(0xFFFE2C55), fontSize: 14),
          ),
        ),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Conversation tile
// ---------------------------------------------------------------------------

class _ConversationTile extends StatelessWidget {
  final ConversationEntity conversation;
  final String currentUserId;
  final VoidCallback onTap;

  const _ConversationTile({
    required this.conversation,
    required this.currentUserId,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final name = conversation.displayName(currentUserId);
    final avatarUrl = conversation.avatarUrl(currentUserId);
    final isOnline = conversation.isOnline(currentUserId);
    final hasUnread = conversation.unreadCount > 0;

    return InkWell(
      onTap: onTap,
      splashColor: Colors.white10,
      highlightColor: Colors.white10,
      child: Padding(
        padding:
            const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
        child: Row(
          children: [
            // Avatar with online dot
            Stack(
              children: [
                CircleAvatar(
                  radius: 28,
                  backgroundColor: const Color(0xFF2A2A2A),
                  backgroundImage: avatarUrl != null
                      ? CachedNetworkImageProvider(avatarUrl)
                      : null,
                  child: avatarUrl == null
                      ? Text(
                          name.isNotEmpty ? name[0].toUpperCase() : '?',
                          style: const TextStyle(
                            color: Colors.white,
                            fontSize: 20,
                            fontWeight: FontWeight.w600,
                          ),
                        )
                      : null,
                ),
                if (isOnline)
                  Positioned(
                    right: 2,
                    bottom: 2,
                    child: Container(
                      width: 13,
                      height: 13,
                      decoration: BoxDecoration(
                        color: const Color(0xFF00D68F),
                        shape: BoxShape.circle,
                        border:
                            Border.all(color: Colors.black, width: 2),
                      ),
                    ),
                  ),
              ],
            ),
            const SizedBox(width: 12),
            // Content
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    name,
                    style: TextStyle(
                      color: Colors.white,
                      fontSize: 15,
                      fontWeight: hasUnread
                          ? FontWeight.w700
                          : FontWeight.w400,
                    ),
                  ),
                  const SizedBox(height: 3),
                  Text(
                    _lastMessagePreview(),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      color: hasUnread
                          ? Colors.white70
                          : Colors.white38,
                      fontSize: 13,
                      fontWeight: hasUnread
                          ? FontWeight.w500
                          : FontWeight.w400,
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(width: 8),
            // Trailing: time + badge
            Column(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                Text(
                  _timeAgo(conversation.lastMessageAt),
                  style: TextStyle(
                    color: hasUnread
                        ? const Color(0xFFFE2C55)
                        : Colors.white38,
                    fontSize: 12,
                  ),
                ),
                const SizedBox(height: 4),
                if (hasUnread)
                  Container(
                    constraints: const BoxConstraints(minWidth: 20),
                    height: 20,
                    padding: const EdgeInsets.symmetric(horizontal: 6),
                    decoration: const BoxDecoration(
                      color: Color(0xFFFE2C55),
                      borderRadius:
                          BorderRadius.all(Radius.circular(10)),
                    ),
                    child: Center(
                      child: Text(
                        conversation.unreadCount > 99
                            ? '99+'
                            : '${conversation.unreadCount}',
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 11,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                    ),
                  )
                else
                  const SizedBox(height: 20),
              ],
            ),
          ],
        ),
      ),
    );
  }

  String _lastMessagePreview() {
    final msg = conversation.lastMessage;
    if (msg == null) return 'No messages yet';
    switch (msg.type) {
      case MessageType.image:
        return '📷 Photo';
      case MessageType.video:
        return '🎥 Video';
      case MessageType.voice:
        return '🎤 Voice';
      case MessageType.gift:
        return '🎁 Gift';
      case MessageType.system:
        return msg.content;
      case MessageType.text:
        final c = msg.content;
        return c.length > 40 ? '${c.substring(0, 40)}…' : c;
    }
  }

  String _timeAgo(DateTime? dt) {
    if (dt == null) return '';
    final diff = DateTime.now().difference(dt);
    if (diff.inMinutes < 1) return 'now';
    if (diff.inHours < 1) return '${diff.inMinutes}m';
    if (diff.inDays < 1) return '${diff.inHours}h';
    if (diff.inDays < 7) return '${diff.inDays}d';
    return '${dt.day}/${dt.month}';
  }
}

// ---------------------------------------------------------------------------
// Empty state
// ---------------------------------------------------------------------------

class _EmptyState extends StatelessWidget {
  final bool hasSearch;
  const _EmptyState({required this.hasSearch});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 40),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Text('💬', style: TextStyle(fontSize: 56)),
            const SizedBox(height: 16),
            Text(
              hasSearch
                  ? 'No conversations match your search.'
                  : 'No messages yet.\nFollow creators and start chatting!',
              style: const TextStyle(
                color: Colors.white54,
                fontSize: 15,
                height: 1.5,
              ),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }
}