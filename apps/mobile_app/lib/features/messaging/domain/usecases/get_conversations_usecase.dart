import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/features/messaging/domain/entities/conversation_entity.dart';
import 'package:tiktok_clone/features/messaging/domain/repositories/message_repository.dart';

class GetConversationsUseCase {
  final MessageRepository _repository;

  const GetConversationsUseCase(this._repository);

  Future<Either<String, (List<ConversationEntity>, String?)>> call({
    String? cursor,
  }) {
    return _repository.getConversations(cursor: cursor);
  }
}
