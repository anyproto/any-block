# Any-Block
Protocol describing data structures used in Anytype software.

## Description
We use [Protocol Buffers](https://en.wikipedia.org/wiki/Protocol_Buffers) to efficiently describe structured data in a binary format for network communication and storage. It offers smaller message sizes and faster serialization/deserialization than JSON or XML. 

Protobuf facilitates data exchange between different systems written in different languages, while also providing automatic code generation for various programming languages.

- `models.proto` describes the base data structures used to represent objects and their components. 
- `changes.proto` outlines CRDT-changes of objects and their blocks. Changes related to block updates are linked to events from `events.proto`. 
- `events.proto` describes the events about the changes of objects and blocks. These events are used to notify clients and also serve as CDRT changes to be stored in an object's tree.

## Contribution
Thank you for your desire to develop Anytype together. 

Currently, we're not ready to accept PRs, but we will in the nearest future.

Follow us on [Github](https://github.com/anyproto) and join the [Contributors Community](https://github.com/orgs/anyproto/discussions).

---
Made by Any — a Swiss association 🇨🇭

Licensed under [MIT License](./LICENSE).