import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/features/messaging/domain/entities/message_entity.dart';
import 'package:tiktok_clone/features/messaging/domain/repositories/message_repository.dart';

class GetMessagesUseCase {
  final MessageRepository _repository;

  const GetMessagesUseCase(this._repository);

  Future<Either<String, (List<MessageEntity>, String?)>> call(
    String conversationId, {
    String? cursor,
  }) {
    return _repository.getMessages(conversationId, cursor: cursor);
  }
}
